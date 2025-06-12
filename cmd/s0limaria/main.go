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
	"strconv"
	"strings"

	"github.com/nsupc/eurogo/client"
	"github.com/nsupc/eurogo/models"
	slogbetterstack "github.com/samber/slog-betterstack"
	"github.com/tmaxmax/go-sse"
)

func main() {
	config, err := config.New("./config.yml")
	if err != nil {
		log.Fatal(err)
	}

	var logger *slog.Logger
	var logLevel slog.Level

	switch config.Log.Level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	if config.Log.Token != "" && config.Log.Endpoint != "" {
		logger = slog.New(slogbetterstack.Option{
			Token:    config.Log.Token,
			Endpoint: config.Log.Endpoint,
			Level:    logLevel,
		}.NewBetterstackHandler())
	} else {
		logger = slog.Default()
	}

	slog.SetDefault(logger)
	slog.SetLogLoggerLevel(logLevel)

	client := client.New(config.Eurocore.User, config.Eurocore.Password, config.Eurocore.Url)
	sseClient := nsse.New()
	nsClient := ns.New(config.User, config.Ratelimit)

	formatted := make([]string, len(config.Targets))
	for i, target := range config.Targets {
		formatted[i] = fmt.Sprintf("region:%s", target.Region)
	}

	waRegex := regexp.MustCompile(`^@@(.*)@@ was admitted to the World Assembly.?$`)
	moveRegex := regexp.MustCompile(`^@@(.+)@@ relocated from %%%%.+%%%% to %%%%(.+)%%%%.?$`)

	url := fmt.Sprintf("https://www.nationstates.net/api/%s", strings.Join(formatted, "+"))

	sseClient.Subscribe(url, func(e sse.Event) {
		event := nsse.Event{}

		err := json.Unmarshal([]byte(e.Data), &event)
		if err != nil {
			slog.Error("unable to marshal event", slog.Any("error", err))
			return
		}

		nationName := ""

		matches := waRegex.FindStringSubmatch(event.Text)

		if len(matches) > 0 {
			nationName = matches[1]
			slog.Info("admit in target region", slog.String("nation", nationName))
		}

		if nationName == "" {
			matches = moveRegex.FindStringSubmatch(event.Text)

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

		nation, err := nsClient.RecruitmentEligible(nationName, config.Region)
		if err != nil {
			slog.Error("unable to retrieve nation details", slog.Any("error", err))
			return
		}

		// we have already validated that target exists
		target, _ := config.Get(nation.Region)

		if nation.CanRecruit {
			telegram := models.Telegram{
				Recipient: nationName,
				Sender:    config.User,
				Id:        strconv.Itoa(target.Telegram.Id),
				Secret:    target.Telegram.Key,
				Type:      "recruitment",
			}

			go func() {
				slog.Info("sending telegram", slog.String("recipient", telegram.Recipient))
				err = client.SendTelegram(telegram)
				if err != nil {
					slog.Error("unable to send telegram", slog.Any("error", err))
				}
			}()
		} else {
			slog.Info("nation not eligible for recruitment")
		}
	})
}
