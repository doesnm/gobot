package main

import "fmt"
import "os"
import "os/signal"
import "syscall"
import "database/sql"
import "flag"
import "time"
import "github.com/bwmarrin/discordgo"
import _ "github.com/lib/pq"
func replyEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed){
	_, err := s.InteractionResponseEdit(s.State.User.ID,i.Interaction,&discordgo.WebhookEdit{
		Embeds: []*discordgo.MessageEmbed{embed},
	})
	if err != nil {
			fmt.Println(err)
		}
}


func MemberHasPermission(s *discordgo.Session, guildID string, userID string, permission int64) (bool, error) {
	member, err := s.State.Member(guildID, userID)
	if err != nil {
		if member, err = s.GuildMember(guildID, userID); err != nil {
			return false, err
		}
	}
	for _, roleID := range member.Roles {
		role, err := s.State.Role(guildID, roleID)
		if err != nil {
			return false, err
		}
		if role.Permissions&permission != 0 || role.Permissions&discordgo.PermissionAdministrator != 0{
			return true, nil
		}
	}

	return false, nil
}

var commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate, db *sql.DB){
	"help": func(s *discordgo.Session, i *discordgo.InteractionCreate, db *sql.DB){
	  
		replyEmbed(s,i,&discordgo.MessageEmbed{
			Title: "Command List",
			Description: "`/help`, `/avatar`, `/clear`, `/autorole`, `/mute`",
		})
	},
	"avatar": func(s *discordgo.Session, i *discordgo.InteractionCreate, db *sql.DB){
		var user *discordgo.User
		if len(i.ApplicationCommandData().Options) < 1 {
			user = i.Member.User
		}else{
		user = i.ApplicationCommandData().Options[0].UserValue(s)
		}
		if user != nil {
			replyEmbed(s,i,&discordgo.MessageEmbed{
				Title: user.Username + "#" + user.Discriminator + "'s avatar",
				Image: &discordgo.MessageEmbedImage{
					URL: user.AvatarURL("1024"),
				},
			})
			return
		}
		replyEmbed(s,i,&discordgo.MessageEmbed{
			Description: ":x: User not found",
		})
	},
	"clear": func(s *discordgo.Session, i *discordgo.InteractionCreate, db *sql.DB){
		bot_has, err := MemberHasPermission(s,i.GuildID,s.State.User.ID,discordgo.PermissionManageMessages)
		if err != nil {
			fmt.Println(err)
			return
		}
		if bot_has == false {
			replyEmbed(s,i,&discordgo.MessageEmbed{
				Description: ":x: Bot doesn't have `MANAGE_MESSAGES` permission",
			})
			return
		}
		has, err := MemberHasPermission(s,i.GuildID,i.Member.User.ID,discordgo.PermissionManageMessages)
		if err != nil {
			fmt.Println(err)
			return
		}
		if has == false {
			replyEmbed(s,i,&discordgo.MessageEmbed{
				Description: ":x: You doesn't have `MANAGE_MESSAGES` permission",
			})
			return
		}
		var count int = int(i.ApplicationCommandData().Options[0].IntValue()) + 1
		if count < 0 || count > 100 {
			replyEmbed(s,i,&discordgo.MessageEmbed{
				Description: ":x: Count cannot be less than 0 or greater than 100",
			})
			return
		}
		messages, err := s.ChannelMessages(i.ChannelID,count,"","","")
		if(err != nil){
			replyEmbed(s,i,&discordgo.MessageEmbed{
				Description: ":x: Bot doesn't have `VIEW_HISTORY` permission",
			})
			return
		}
		ids := make([]string,0,count)
		for index := range messages {
			ids = append(ids,messages[index].ID)
		}
	err =	s.ChannelMessagesBulkDelete(i.ChannelID,ids)
	if err != nil {
	  replyEmbed(s,i, &discordgo.MessageEmbed{
	  	Description: ":x: Bot doesn't have `MANAGE_MESSAGES` permission",
	  })
	  return
	}
	},
	"autorole": func(s *discordgo.Session, i *discordgo.InteractionCreate, db *sql.DB){
		has,_ := MemberHasPermission(s,i.GuildID, i.Member.User.ID,discordgo.PermissionManageRoles)
		if has == false {
			replyEmbed(s,i,&discordgo.MessageEmbed{
			Description: ":x: You doesn't have `MANAGE_ROLES` permission",
		})
			return 
		}
		if len(i.ApplicationCommandData().Options) < 1 {
			var autorole string;
		  err := db.QueryRow("select autorole from guilds where id=$1",i.GuildID).Scan(&autorole)
		  if err != nil {
		  	fmt.Print(err)
		  }
		  if autorole == "" {
		  	replyEmbed(s,i, &discordgo.MessageEmbed{
		  	Description: "Autorole is disabled on this server",
		  })
		  	return
		  }
		  replyEmbed(s,i, &discordgo.MessageEmbed{
		  	Description: "Autorole on this server: <@&" + autorole + ">",
		  })
		  return
		}else{
			role := i.ApplicationCommandData().Options[0].RoleValue(s,i.GuildID)
			var autorole string;
		  err := db.QueryRow("select autorole from guilds where id=$1",i.GuildID).Scan(&autorole)
		  _,err = db.Exec("update guilds set autorole = $1 where id = $2", role.ID,i.GuildID)
		  if err != nil {
		  	fmt.Println(err)
		  	return
		  }
			if autorole == role.ID {
				_, err = db.Exec("update guilds set autorole = null where id = $1",i.GuildID)
				if err != nil {
					fmt.Println(err)
					return
				}
				replyEmbed(s,i, &discordgo.MessageEmbed{
		  	Description: ":white_check_mark: Autorole is disabled on this server",
		  })
				return
			}
			replyEmbed(s,i, &discordgo.MessageEmbed{
		  	Description: ":white_check_mark: Autorole on this server is changed to " + role.Mention(),
		  })
		}
	},
	"test": func(s *discordgo.Session, i *discordgo.InteractionCreate, db *sql.DB){
		replyEmbed(s,i,&discordgo.MessageEmbed{
			Description: "Tested!",
		})
	},
	"mute": func(s *discordgo.Session, i *discordgo.InteractionCreate, db *sql.DB){
		has,err := MemberHasPermission(s,i.GuildID,i.Member.User.ID,discordgo.PermissionModerateMembers)
		if err != nil {
			print(err)
			return
		}
		if has == false {
			replyEmbed(s,i,&discordgo.MessageEmbed{
				Description: ":x: You doesn't have `TIMEOUT_MEMBERS` permission",
			})
			return
		}
		has,err = MemberHasPermission(s,i.GuildID,s.State.User.ID,discordgo.PermissionModerateMembers)
		if err != nil {
			print(err)
			return
		}
		if has == false {
			replyEmbed(s,i,&discordgo.MessageEmbed{
				Description: ":x: Bot doesn't have `TIMEOUT_MEMBERS` permission",
			})
			return
		}
		user := i.ApplicationCommandData().Options[0].UserValue(s)
		duration, err := time.ParseDuration(i.ApplicationCommandData().Options[1].StringValue())
		if err != nil {
			replyEmbed(s,i,&discordgo.MessageEmbed{
				Description: ":x: Failed to parse time. Example: `10m`",
			})
			return
		}
		until := time.Now().Add(duration)
		err = s.GuildMemberTimeout(i.GuildID,user.ID,&until)
		if err != nil {
			print(err)
			replyEmbed(s,i,&discordgo.MessageEmbed{
				Description: ":x: Failed to timeout. Fatal Error",
			})
			return
		}
		replyEmbed(s,i,&discordgo.MessageEmbed{
			Description: user.Mention() + " has been muted by " + i.Member.Mention(),
		})
	},
}


func main(){
db, err := sql.Open("postgres", "host=192.168.0.105 port=5432 user=postgres password=password dbname=gobot sslmode=disable")
	if err != nil {
		panic(err)
	}
	err = db.Ping()
  if err != nil {
  panic(err)
  }
  var token = "token"
  debug := flag.Bool("debug",false,"enable debug mode")
  flag.Parse()
  if *debug {
  	fmt.Println("Bot running in debug mode")
  	token = "dev token"
  }
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("Error creating session: ",err)
		return
	}
	dg.Identify.Intents = 3
	dg.AddHandler(ready)
	dg.AddHandler(func(s *discordgo.Session, m *discordgo.GuildMemberAdd){
      var autorole string;
		  db.QueryRow("select autorole from guilds where id=$1",m.GuildID).Scan(&autorole)
	     if autorole == "" {
		      return
	     }
	      s.GuildMemberRoleAdd(m.GuildID,m.User.ID,autorole)
	})
	dg.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate){
		if i.GuildID == "" {
			replyEmbed(s,i,&discordgo.MessageEmbed{
				Description: ":x: Slash commands is disabled in DM",
			})
			return
		}
		_, err := db.Exec("insert into guilds (id) values ($1) on conflict do nothing",i.GuildID)
		if err != nil {
			fmt.Println(err)
		}
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		  })
			if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
				h(s,i,db)
			}
		})
	dg.AddHandler(func(s *discordgo.Session, event *discordgo.GuildCreate){
		s.UpdateGameStatus(0,fmt.Sprintf("/help | %d guilds",len(s.State.Guilds)))
	})
	dg.AddHandler(func(s *discordgo.Session, event *discordgo.GuildDelete){
		s.UpdateGameStatus(0,fmt.Sprintf("/help | %d guilds",len(s.State.Guilds)))
	})
	err = dg.Open()
	if err != nil {
		fmt.Println("Error opening session: ",err)
		return
	}
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal,1)
	signal.Notify(sc,syscall.SIGINT,syscall.SIGTERM,os.Interrupt,os.Kill)
	<-sc
	dg.Close()
}

func ready(s *discordgo.Session, event *discordgo.Ready){
	s.UpdateGameStatus(0,fmt.Sprintf("/help | %d guilds",len(s.State.Guilds)))
}
