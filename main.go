// Package botmeans provides a framework for creation of complex high-loaded Telegram bots with rich behaviour
package botmeans

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/jinzhu/gorm"
)

//MeansBot is a body of botmeans framework instance.
type MeansBot struct {
	bot       *tgbotapi.BotAPI
	db        *gorm.DB
	netConfig NetConfig
	tlgConfig TelegramConfig
}

//NetConfig is a MeansBot network config for using with New function
type NetConfig struct {
	ListenIP   string
	ListenPort int16
}

//TelegramConfig is a MeansBot telegram API config for using with New function
type TelegramConfig struct {
	BotToken    string
	WebhookHost string
	SSLCertFile string
	BotName     string
	TemplateDir string
}

//New creates new MeansBot instance
func New(DB *gorm.DB, netConfig NetConfig, tlgConfig TelegramConfig) (*MeansBot, error) {
	if DB == nil {
		return &MeansBot{}, fmt.Errorf("No db connection given")
	}
	bot, err := tgbotapi.NewBotAPI(tlgConfig.BotToken)
	if err != nil {
		return &MeansBot{}, err
	}

	ret := &MeansBot{
		bot:       bot,
		db:        DB,
		netConfig: netConfig,
		tlgConfig: tlgConfig,
	}
	if os.Getenv("BOTMEANS_SET_WEBHOOK") == "TRUE" {

		ret.bot.RemoveWebhook()
		webhookConfig := tgbotapi.NewWebhookWithCert(fmt.Sprintf("https://%v:8443/%v", ret.tlgConfig.WebhookHost, ret.bot.Token),
			ret.tlgConfig.SSLCertFile)
		webhookConfig.MaxConnections = 50
		_, err = ret.bot.SetWebhook(webhookConfig)
		if err != nil {
			return nil, err
		}
		info, _ := ret.bot.GetWebhookInfo()
		log.Printf("Webhook set: %+v", info)
	}

	SessionInitDB(DB)
	BotMessageInitDB(DB)

	return ret, nil
}

//Run starts updates handling. Returns stop chan
func (ui *MeansBot) Run(handlersProvider ActionHandlersProvider) chan interface{} {
	templateDir := ui.tlgConfig.TemplateDir
	botID, _ := strconv.ParseInt(strings.Split(ui.bot.Token, ":")[0], 10, 64)

	sessionFactory := func(base SessionBase) (SessionInterface, error) {
		return SessionLoader(base, ui.db, botID, ui.bot)
	}

	actionFactory := func(
		sessionBase SessionBase,
		sessionFactory SessionFactory,
		getters actionExecuterFactoryConfig,
		out chan Executer,
	) {
		ActionFactory(
			sessionBase,
			sessionFactory,
			getters,
			func(s senderSession) SenderInterface {
				return &Sender{
					session:     s,
					bot:         ui.bot,
					templateDir: templateDir,
					msgFactory:  func() BotMessageInterface { return NewBotMessage(s.ChatId(), ui.db) },
				}
			},
			out,
			handlersProvider,
		)
	}

	aliaser := AliaserFromTemplateDir(templateDir)
	argsParser := func(tgUpdate tgbotapi.Update) Args {
		return ArgsParser(tgUpdate, sessionFactory, aliaser)
	}
	cmdParser := func(tgUpdate tgbotapi.Update) string {
		return CmdParser(tgUpdate, aliaser)
	}
	botMsgFactory := func(chatID int64, msgId int64, callbackID string) BotMessageInterface {
		return BotMessageDBLoader(chatID, msgId, callbackID, ui.db)
	}
	updatesChan := ui.bot.ListenForWebhook("/" + ui.bot.Token)

	go http.ListenAndServe(fmt.Sprintf("%v:%v", ui.netConfig.ListenIP, ui.netConfig.ListenPort), nil)

	actionsChan := createTGUpdatesParser(
		updatesChan,
		parserConfig{
			sessionFactory,
			actionFactory,
			botMsgFactory,
			cmdParser,
			argsParser,
		},
	)
	return RunMachine(actionsChan, time.Minute)
}
