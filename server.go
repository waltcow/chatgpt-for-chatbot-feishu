package main

import (
	"github.com/go-zoox/chatgpt-for-chatbot-feishu/client"
	"strings"
	"time"

	"github.com/go-zoox/chalk"
	"github.com/go-zoox/chatbot-feishu"
	"github.com/go-zoox/core-utils/regexp"
	"github.com/go-zoox/debug"

	"github.com/go-zoox/core-utils/fmt"
	mc "github.com/go-zoox/feishu/message/content"

	feishuEvent "github.com/go-zoox/feishu/event"
	"github.com/go-zoox/logger"
	"github.com/go-zoox/retry"
)

type FeishuBotConfig struct {
	Port              int64
	APIPath           string
	ChatGPTAPIKey     string
	AppID             string
	AppSecret         string
	BotOpenID         string
	EncryptKey        string
	VerificationToken string
	//
	ReportURL string
	//
	SiteURL string
	//
	OpenAIModel string
	//
	FeishuBaseURI string
	//
	ChatGPTContext  string
	ChatGPTLanguage string
	//
	LogsDir string

	ProxyAPIServer string
}

func ServeFeishuBot(cfg *FeishuBotConfig) (err error) {
	logger.Infof("###### Settings START #######")
	logger.Infof("Serve at PORT: %d", cfg.Port)
	logger.Infof("Serve at API_PATH: %s", cfg.APIPath)
	logger.Infof("###### Settings END #######")

	logs := &Logs{
		Dir: cfg.LogsDir,
	}
	if err := logs.Setup(); err != nil {
		return fmt.Errorf("failed to setup logs: %v", err)
	}

	client, err := chatgptclient.New(&chatgptclient.Config{
		APIKey:               cfg.ChatGPTAPIKey,
		ConversationContext:  cfg.ChatGPTContext,
		ConversationLanguage: cfg.ChatGPTLanguage,
		ProxyAPIServer:       cfg.ProxyAPIServer,
	})

	if err != nil {
		return fmt.Errorf("failed to create chatgpt client: %v", err)
	}

	if debug.IsDebugMode() {
		fmt.PrintJSON(map[string]interface{}{
			"cfg": cfg,
		})
	}

	if cfg.SiteURL != "" {
		logger.Infof("")
		logger.Infof("###### Feishu Configuration START #######")
		logger.Infof("# %s：%s", chalk.Red("飞书事件订阅请求地址"), chalk.Green(fmt.Sprintf("%s%s", cfg.SiteURL, cfg.APIPath)))
		logger.Infof("###### Feishu Configuration END #######")
		logger.Infof("")
	}

	feishuchatbot, err := chatbot.New(&chatbot.Config{
		Port:              cfg.Port,
		Path:              cfg.APIPath,
		AppID:             cfg.AppID,
		AppSecret:         cfg.AppSecret,
		VerificationToken: cfg.VerificationToken,
		EncryptKey:        cfg.EncryptKey,
	})

	if err != nil {
		return fmt.Errorf("failed to create feishu chatbot: %v", err)
	}

	feishuchatbot.OnCommand("ping", &chatbot.Command{
		Handler: func(args []string, request *feishuEvent.EventRequest, reply func(content string, msgType ...string) error) error {
			msgType, content, err := mc.
				NewContent().
				Post(&mc.ContentTypePost{
					ZhCN: &mc.ContentTypePostBody{
						Content: [][]mc.ContentTypePostBodyItem{
							{
								{
									Tag:      "text",
									UnEscape: true,
									Text:     "pong",
								},
							},
						},
					},
				}).
				Build()
			if err != nil {
				return fmt.Errorf("failed to build content: %v", err)
			}
			if err := reply(string(content), msgType); err != nil {
				return fmt.Errorf("failed to reply: %v", err)
			}

			return nil
		},
	})

	feishuchatbot.OnMessage(func(text string, request *feishuEvent.EventRequest, reply func(content string, msgType ...string) error) error {

		user := request.Sender().SenderID.UserID

		textMessage := strings.TrimSpace(text)

		if textMessage == "" {
			logger.Infof("ignore empty message")
			return nil
		}

		var question string
		// group chat
		if request.IsGroupChat() {
			// @
			if ok := regexp.Match("^@_user_1", textMessage); ok {
				for _, mention := range request.Event.Message.Mentions {
					if mention.Key == "@_user_1" && mention.ID.OpenID == cfg.BotOpenID {
						question = textMessage[len("@_user_1"):]
						question = strings.TrimSpace(question)
						break
					}
				}
			}
		} else if request.IsP2pChat() {
			question = textMessage
		}

		question = strings.TrimSpace(question)

		if question == "" {
			logger.Infof("ignore empty question message")
			return nil
		}

		go func() {
			logger.Infof("%s 问：%s", user, question)
			var err error

			conversation, err := client.GetOrCreateConversation(request.ChatID(), &chatgptclient.ConversationConfig{
				MaxMessages: 50,
			})

			if err != nil {
				logger.Errorf("failed to get or create conversation by ChatID %s", request.ChatID())
				return
			}

			if err := conversation.IsQuestionAsked(request.Event.Message.MessageID); err != nil {
				logger.Warnf("duplicated event(id: %s): %v", request.Event.Message.MessageID, err)
				return
			}

			var answer []byte
			err = retry.Retry(func() error {

				answer, err = conversation.Ask([]byte(question), &chatgptclient.ConversationAskConfig{
					ID:   request.Event.Message.MessageID,
					User: user,
				})
				if err != nil {
					logger.Errorf("failed to request answer: %v", err)
					return fmt.Errorf("failed to request answer: %v", err)
				}

				return nil
			}, 5, 3*time.Second)

			if err != nil {
				logger.Errorf("failed to get answer: %v", err)
				msgType, content, err := mc.
					NewContent().
					Text(&mc.ContentTypeText{
						Text: "ChatGPT 繁忙，请稍后重试",
					}).
					Build()
				if err != nil {
					logger.Errorf("failed to build content: %v", err)
					return
				}
				if err := reply(string(content), msgType); err != nil {
					return
				}
				return
			}

			logger.Infof("ChatGPT 答：%s", answer)
			responseMessage := string(answer)

			if request.IsGroupChat() {
				responseMessage = fmt.Sprintf("%s\n-------------\n%s", question, answer)
			}

			msgType, content, err := mc.
				NewContent().
				Post(&mc.ContentTypePost{
					ZhCN: &mc.ContentTypePostBody{
						Content: [][]mc.ContentTypePostBodyItem{
							{
								{
									Tag:      "text",
									UnEscape: true,
									Text:     responseMessage,
								},
							},
						},
					},
				}).
				Build()

			if err != nil {
				logger.Errorf("failed to build content: %v", err)
				return
			}

			if err := reply(content, msgType); err != nil {
				logger.Errorf("failed to reply: %v", err)
				return
			}
		}()

		return nil
	})

	return feishuchatbot.Run()
}
