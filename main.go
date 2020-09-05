package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	ubot "github.com/UBotPlatform/UBot.Common.Go"
	"github.com/bwmarrin/discordgo"
)

var event *ubot.AccountEventEmitter
var dg *discordgo.Session

func getGroupName(id string) (string, error) {
	st, err := dg.State.Channel(id)
	if err != nil {
		st, err = dg.Channel(id)
		if err != nil {
			return "", err
		}
	}
	return st.Name, nil
}
func getUserName(id string) (string, error) {
	st, err := dg.User(id)
	if err != nil {
		return "", err
	}
	return st.Username, nil
}
func sendChatMessage(msgType ubot.MsgType, source string, target string, message string) error {
	entities := ubot.ParseMsg(message)
	var rawMsg strings.Builder
	for _, entity := range entities {
		switch entity.Type {
		case "text":
			data := entity.Data
			data = strings.ReplaceAll(data, "\\", "\\\\")
			data = strings.ReplaceAll(data, ">", "\\<")
			data = strings.ReplaceAll(data, "<", "\\>")
			rawMsg.WriteString(data)
		case "at":
			rawMsg.WriteString("<@!")
			rawMsg.WriteString(entity.Data)
			rawMsg.WriteByte('>')
		default:
			rawMsg.WriteString("[不支持的消息类型]")
		}
	}
	_, err := dg.ChannelMessageSend(source, rawMsg.String())
	return err
}

func convertFromDiscordMsg(m *discordgo.Message) string {
	var builder ubot.MsgBuilder
	rawMsg := m.Content
	start, count := 0, 0
	for i := 0; i < len(rawMsg); i++ {
		switch rawMsg[i] {
		case '\\':
			if i+1 >= len(rawMsg) {
				count++
				break
			}
			switch rawMsg[i+1] {
			case '<':
				fallthrough
			case '>':
				fallthrough
			case '\\':
				i++
				builder.WriteString(rawMsg[start : start+count])
				builder.WriteString(rawMsg[i : i+1])
				start = i + 1
				count = 0
			default:
				count++
			}
		case '<':
			if i+4 < len(rawMsg) && rawMsg[i+1] == '@' && rawMsg[i+2] == '!' {
				var j int
				for j = i + 3; j < len(rawMsg) && rawMsg[j] >= '0' && rawMsg[j] <= '9'; j++ {
				}
				if j < len(rawMsg) && rawMsg[j] == '>' {
					builder.WriteString(rawMsg[start : start+count])
					builder.WriteEntity(ubot.MsgEntity{Type: "at", Data: rawMsg[i+3 : j]})
					i = j
					start = i + 1
					count = 0
				}
				break
			}
			count++
		default:
			count++
		}
	}
	builder.WriteString(rawMsg[start : start+count])
	return builder.String()
}
func removeMember(source string, target string) error {
	st, err := dg.State.Channel(source)
	if err != nil {
		st, err = dg.Channel(source)
		if err != nil {
			return err
		}
		_ = dg.State.ChannelAdd(st)
	}
	err = dg.GuildMemberDelete(st.GuildID, target)
	return err
}
func shutupMember(source string, target string, duration int) error {
	return errors.New("not supported")
}
func shutupAllMember(source string, shutupSwitch bool) error {
	return errors.New("not supported")
}

func getMemberName(source string, target string) (string, error) {
	st, err := dg.State.Member(source, target)
	if err != nil {
		return getUserName(target) // fallback
	}
	if st.Nick != "" {
		return st.Nick, nil
	}
	if st.User != nil {
		return st.User.Username, nil
	}
	return getUserName(target) // fallback
}

func getUserAvatar(id string) (string, error) {
	u, err := dg.User(id)
	if err != nil {
		return "", err
	}
	return u.AvatarURL(""), nil
}
func getSelfID() (string, error) {
	return dg.State.User.ID, nil
}
func getPlatformID() (string, error) {
	return "Discord", nil
}
func getGroupList() ([]string, error) {
	var r []string
	for _, guild := range dg.State.Guilds {
		for _, channel := range guild.Channels {
			if channel.Type == discordgo.ChannelTypeGuildText {
				r = append(r, channel.ID)
			}
		}
	}
	return r, nil
}
func getMemberList(id string) ([]string, error) {
	var r []string
	channel, err := dg.State.Channel(id)
	if err != nil {
		channel, err = dg.Channel(id)
		if err != nil {
			return nil, err
		}
	}
	guild, err := dg.State.Guild(channel.GuildID)
	if err != nil {
		return nil, err
	}
	for _, member := range guild.Members {
		r = append(r, member.User.ID)
	}
	return r, nil
}

func onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}
	var msgType ubot.MsgType
	st, err := dg.State.Channel(m.ChannelID)
	if err != nil {
		st, err = dg.Channel(m.ChannelID)
		if err != nil {
			return
		}
		_ = dg.State.ChannelAdd(st)
	}
	if st.Type == discordgo.ChannelTypeDM {
		msgType = ubot.PrivateMsg
	} else {
		msgType = ubot.GroupMsg
	}
	_ = event.OnReceiveChatMessage(msgType, m.ChannelID, m.Author.ID, convertFromDiscordMsg(m.Message), ubot.MsgInfo{
		ID: m.Message.ID,
	})
}

func onGuildMemberAdd(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	guild, err := dg.State.Guild(m.GuildID)
	if err != nil {
		return
	}
	for _, channel := range guild.Channels {
		_ = event.OnMemberJoined(channel.ID, m.User.ID, "")
	}
}

func onGuildMemberRemove(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	guild, err := dg.State.Guild(m.GuildID)
	if err != nil {
		return
	}
	for _, channel := range guild.Channels {
		_ = event.OnMemberLeft(channel.ID, m.User.ID)
	}
}

func main() {
	var err error
	dg, err = discordgo.New("Bot " + os.Args[3])
	ubot.AssertNoError(err)
	err = dg.Open()
	if err != nil {
		fmt.Println("Failed to login to discord:", err)
		os.Exit(111)
	}
	err = ubot.HostAccount("Discord Bot", func(e *ubot.AccountEventEmitter) *ubot.Account {
		event = e
		dg.AddHandler(onMessageCreate)
		dg.AddHandler(onGuildMemberAdd)
		dg.AddHandler(onGuildMemberRemove)
		return &ubot.Account{
			GetGroupName:    getGroupName,
			GetUserName:     getUserName,
			SendChatMessage: sendChatMessage,
			RemoveMember:    removeMember,
			ShutupMember:    shutupMember,
			ShutupAllMember: shutupAllMember,
			GetMemberName:   getMemberName,
			GetUserAvatar:   getUserAvatar,
			GetSelfID:       getSelfID,
			GetPlatformID:   getPlatformID,
			GetGroupList:    getGroupList,
			GetMemberList:   getMemberList,
		}
	})
	ubot.AssertNoError(err)
	_ = dg.Close()
}
