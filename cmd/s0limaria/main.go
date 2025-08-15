package main

import (
	"fmt"
	"log"
	"log/slog"
	"regexp"
	"s0limaria/pkg/config"
	"s0limaria/pkg/ns"
	"s0limaria/pkg/sse"
	"strconv"
	"strings"

	"github.com/nsupc/eurogo/client"
	"github.com/nsupc/eurogo/telegrams"
)

func main() {
	config, err := config.New()
	if err != nil {
		log.Fatal(err)
	}

	client := client.New(config.Eurocore.User, config.Eurocore.Password, config.Eurocore.Url)

	nsClient := ns.New(config.User, config.Ratelimit)

	formatted := make([]string, len(config.Targets))
	for i, target := range config.Targets {
		formatted[i] = fmt.Sprintf("region:%s", target.Region)
	}

	waRegex := regexp.MustCompile(`^@@(.*)@@ was admitted to the World Assembly.?$`)
	moveRegex := regexp.MustCompile(`^@@(.+)@@ relocated from %%%%.+%%%% to %%%%(.+)%%%%.?$`)

	url := fmt.Sprintf("https://www.nationstates.net/api/%s", strings.Join(formatted, "+"))

	sseClient := sse.New(url)
	sseClient.Subscribe(func(e sse.Event) {
		nationName := ""

		matches := waRegex.FindStringSubmatch(e.Text)

		if len(matches) > 0 {
			nationName = matches[1]
			slog.Info("admit in target region", slog.String("nation", nationName))
		}

		if nationName == "" {
			matches = moveRegex.FindStringSubmatch(e.Text)

			if len(matches) > 0 {
				regionName := matches[2]

				if _, exists := config.Get(regionName); exists {
					nationName = matches[1]
					slog.Info("move to target region", slog.String("nation", nationName))
				}
			}
		}

		if nationName == "" {
			return
		}

		go func() {
			nation, err := nsClient.RecruitmentEligible(nationName, config.Region)
			if err != nil {
				slog.Error("unable to retrieve nation details", slog.Any("error", err))
				return
			}

			// we have already validated that target exists
			target, _ := config.Get(nation.Region)

			if nation.CanRecruit {
				tmpl, err := client.GetTemplate(target.Template)
				if err != nil {
					slog.Error("unable to retrieve template", slog.Any("error", err))
					return
				}

				telegram := telegrams.New(tmpl.Nation, nationName, strconv.Itoa(tmpl.Tgid), tmpl.Key, telegrams.Recruitment)

				slog.Info("sending telegram", slog.String("recipient", telegram.Recipient))
				err = client.SendTelegram(telegram)
				if err != nil {
					slog.Error("unable to send telegram", slog.Any("error", err))
					return
				}

				slog.Info("telegram sent")
			} else {
				slog.Info("nation not eligible for recruitment")
			}
		}()

	})
}
