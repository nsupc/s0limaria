package main

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"regexp"
	"s0limaria/pkg/config"
	"s0limaria/pkg/ns"
	nsse "s0limaria/pkg/sse"
	"slices"
	"strconv"
	"strings"

	"github.com/nsupc/eurogo/client"
	"github.com/nsupc/eurogo/models"
	"github.com/tmaxmax/go-sse"
)

func main() {
	config, err := config.New("./config.yml")
	if err != nil {
		log.Fatal(err)
	}

	client := client.New(config.EurocoreUser, config.EurocorePassword, config.EurocoreUrl)
	sseClient := nsse.New()
	nsClient := ns.New(config.User, config.Ratelimit)

	formatted := make([]string, len(config.Targets))
	for i, region := range config.Targets {
		formatted[i] = fmt.Sprintf("region:%s", region)
	}

	waRegex := regexp.MustCompile(`^@@(.*)@@ was admitted to the World Assembly.?$`)
	moveRegex := regexp.MustCompile(`^@@(.+)@@ relocated from %%%%.+%%%% to %%%%(.+)%%%%$`)

	url := fmt.Sprintf("https://www.nationstates.net/api/%s", strings.Join(formatted, "+"))

	sseClient.Subscribe(url, func(e sse.Event) {
		event := nsse.Event{}

		err := json.Unmarshal([]byte(e.Data), &event)
		if err != nil {
			slog.Error("unable to marshal event", slog.Any("error", err))
			return
		}

		matches := waRegex.FindStringSubmatch(event.Text)

		if len(matches) > 0 {
			nationName := matches[1]
			slog.Info("admit in target region", slog.String("nation", nationName))

			telegram := models.Telegram{
				Recipient: nationName,
				Sender:    config.User,
				Id:        strconv.Itoa(config.Telegram.Id),
				Secret:    config.Telegram.Key,
				Type:      "recruitment",
			}

			go client.SendTelegram(telegram)

			return
		}

		matches = moveRegex.FindStringSubmatch(event.Text)

		if len(matches) > 0 {
			nationName := matches[1]
			regionName := matches[2]

			if slices.Contains(config.Targets, regionName) {
				canRecruit, err := nsClient.RecruitmentEligible(nationName, regionName)
				if err != nil {
					slog.Error("unable to retrieve nation details", slog.Any("error", err))
					return
				}

				if canRecruit {
					telegram := models.Telegram{
						Recipient: nationName,
						Sender:    config.User,
						Id:        strconv.Itoa(config.Telegram.Id),
						Secret:    config.Telegram.Key,
						Type:      "recruitment",
					}

					go client.SendTelegram(telegram)
				}

				return
			}
		}
	})
}
