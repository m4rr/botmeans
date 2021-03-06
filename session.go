package botmeans

import (
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/jinzhu/gorm"
	"strings"
	"time"
)

//SessionBase passes core session identifiers
type SessionBase struct {
	TelegramUserID   int64  `sql:"index"`
	TelegramUserName string `sql:"index"`
	TelegramChatID   int64  `sql:"index"`
	hasCome          bool
	hasLeft          bool
}

//Session represents the user in chat.
type Session struct {
	SessionBase
	ID        int64  `sql:"index;unique"`
	UserData  string `sql:"type:jsonb"`
	db        *gorm.DB
	FirstName string
	LastName  string
	ChatName  string
	CreatedAt time.Time
	isNew     bool
}

//IsNew should return true if the session has not been saved yet
func (session *Session) IsNew() bool {
	return session.isNew
}

//HasLeft returns true if the user has gone from chat
func (session *Session) HasLeft() bool {
	return session.hasLeft
}

//HasCome returns true if the user has come to chat
func (session *Session) HasCome() bool {
	return session.hasCome
}

//IsOneToOne should return true if the session represents one-to-one chat with bot
func (session *Session) IsOneToOne() bool {
	return session.TelegramChatID == session.TelegramUserID
}

//ChatId returns chat id
func (session *Session) ChatId() int64 {
	return session.TelegramChatID
}

func (session *Session) UserId() int64 {
	return session.TelegramUserID
}

//SetData sets internal UserData field to JSON representation of given value
func (session *Session) SetData(value interface{}) {
	if session.db != nil {
		s := Session{}
		if session.db.Where("id=?", session.ID).First(&s).Error == nil {
			session.UserData = s.UserData
		}
	}
	session.UserData = serialize(session.UserData, value)
	session.Save()
}

//GetData extracts internal UserData field to given value
func (session *Session) GetData(value interface{}) {
	deserialize(session.UserData, value)
}

//UserName returns name of the user of this session
func (session *Session) UserName() string {
	s := strings.TrimSpace(session.FirstName + " " + session.LastName)
	if s == "" {
		s = session.TelegramUserName
	}
	return s
}

//ChatName returns name of the chat of this session
func (session *Session) ChatTitle() string {
	return session.ChatName
}

func (session *Session) Id() int64 {
	return session.ID
}

//Save saves the session to sql table
func (session *Session) Save() error {
	if session.db != nil {
		if err := session.db.Save(session).Error; err == nil {
			session.isNew = false
			return nil
		} else {
			return err
		}
	}
	return fmt.Errorf("db not set")
}

//Locale returns the locale for this user
func (session *Session) Locale() string {
	type Locale string

	var lo Locale
	session.GetData(&lo)
	return string(lo)
}

func (session *Session) SetLocale(locale string) {
	type Locale string
	var lo Locale = Locale(locale)
	session.SetData(lo)
}

//String represents the session as string
func (session *Session) String() string {

	return fmt.Sprintf("UserID: %v, UserName: %v, ChatID: %v, New: %v, Come: %v, Left: %v, Data: %v, Name: %v %v, Locale: %v",
		session.TelegramUserID,
		session.TelegramUserName,
		session.TelegramChatID,
		session.isNew,
		session.hasCome,
		session.hasLeft,
		session.UserData,
		session.FirstName,
		session.LastName,
		session.Locale(),
	)
}

//SessionInitDB creates sql table for Session
func SessionInitDB(db *gorm.DB) {
	db.AutoMigrate(&Session{})
}

//SessionLoader creates the session and loads the data if the session exists
func SessionLoader(base SessionBase, db *gorm.DB, BotID int64, api *tgbotapi.BotAPI) (SessionInterface, error) {
	TelegramUserID := base.TelegramUserID
	TelegramUserName := base.TelegramUserName
	TelegramChatID := base.TelegramChatID
	if TelegramUserID == 0 && TelegramUserName == "" {
		return nil, fmt.Errorf("Invalid session IDs")
	}
	//TODO!
	if TelegramUserID == BotID {
		return nil, fmt.Errorf("Cannot create the session for myself")
	}
	session := &Session{}
	session.db = db
	found := !db.Where("((telegram_user_id=? and telegram_user_id!=0) or (telegram_user_name=? and telegram_user_name!='')) and telegram_chat_id=?", TelegramUserID, TelegramUserName, TelegramChatID).
		First(session).RecordNotFound()
	err := fmt.Errorf("Unknown")
	if api != nil && (!found || session.FirstName == "" && session.LastName == "") {
		if chatMember, err := api.GetChatMember(tgbotapi.ChatConfigWithUser{TelegramChatID, "", int(TelegramUserID)}); err == nil {
			session.FirstName = chatMember.User.FirstName
			session.LastName = chatMember.User.LastName
		}
	}
	if !found {
		session.isNew = true
		session.TelegramChatID = TelegramChatID
		session.TelegramUserID = TelegramUserID
		session.TelegramUserName = TelegramUserName
		session.CreatedAt = time.Now()
		if api != nil {
			if chat, err := api.GetChat(tgbotapi.ChatConfig{ChatID: session.TelegramChatID}); err == nil {
				session.ChatName = chat.Title
			}
		}
		session.UserData = "{}"
		err = nil

	}
	err = nil
	session.hasLeft = base.hasLeft
	session.hasCome = base.hasCome
	session.TelegramUserID = base.TelegramUserID
	session.TelegramUserName = base.TelegramUserName

	return session, err
}

type NewSessionCreator func(chatId int64, username string) (SessionInterface, error)

//SessionInterface defines the user session
type SessionInterface interface {
	ChatIdentifier
	UserIdentifier
	PersistentSaver
	DataGetSetter
	IsNew() bool
	HasLeft() bool
	HasCome() bool
	Locale() string
	UserName() string
	Identifiable
	SetLocale(string)
	ChatTitle() string
	IsOneToOne() bool
}
