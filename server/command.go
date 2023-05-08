package main

import (
	"encoding/base32"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/gosimple/slug"
	"github.com/mattermost/mattermost-plugin-api/experimental/command"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/utils"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin"
	"github.com/pborman/uuid"
)

const msteamsCommand = "msteams-sync"

func (p *Plugin) createMsteamsSyncCommand() *model.Command {
	iconData, err := command.GetIconData(p.API, "assets/msteams-sync-icon.svg")
	if err != nil {
		p.API.LogError("Unable to get the msteams icon for the slash command")
	}

	return &model.Command{
		Trigger:              msteamsCommand,
		AutoComplete:         true,
		AutoCompleteDesc:     "Manage synced channels between MS Teams and Mattermost",
		AutoCompleteHint:     "[command]",
		Username:             botUsername,
		DisplayName:          botDisplayName,
		AutocompleteData:     getAutocompleteData(),
		AutocompleteIconData: iconData,
	}
}

func (p *Plugin) cmdError(userID string, channelID string, detailedError string) (*model.CommandResponse, *model.AppError) {
	p.API.SendEphemeralPost(userID, &model.Post{
		Message:   detailedError,
		UserId:    p.userID,
		ChannelId: channelID,
	})
	return &model.CommandResponse{}, nil
}

func (p *Plugin) sendBotEphemeralPost(userID, channelID, message string) {
	p.API.SendEphemeralPost(userID, &model.Post{
		Message:   message,
		UserId:    p.userID,
		ChannelId: channelID,
	})
}

func getAutocompleteData() *model.AutocompleteData {
	cmd := model.NewAutocompleteData(msteamsCommand, "[command]", "Manage MS Teams linked channels")

	link := model.NewAutocompleteData("link", "[msteams-team-id] [msteams-channel-id]", "Link current channel to a MS Teams channel")
	link.AddDynamicListArgument("[msteams-team-id]", getAutocompletePath("teams"), true)
	link.AddDynamicListArgument("[msteams-channel-id]", getAutocompletePath("channels"), true)
	cmd.AddCommand(link)

	unlink := model.NewAutocompleteData("unlink", "", "Unlink the current channel from the MS Teams channel")
	cmd.AddCommand(unlink)

	show := model.NewAutocompleteData("show", "", "Show MS Teams linked channel")
	cmd.AddCommand(show)

	connect := model.NewAutocompleteData("connect", "", "Connect your Mattermost account to your MS Teams account")
	cmd.AddCommand(connect)

	disconnect := model.NewAutocompleteData("disconnect", "", "Disconnect your Mattermost account from your MS Teams account")
	cmd.AddCommand(disconnect)

	connectBot := model.NewAutocompleteData("connect-bot", "", "Connect the bot account (only system admins can do this)")
	connectBot.RoleID = model.SystemAdminRoleId
	cmd.AddCommand(connectBot)

	disconnectBot := model.NewAutocompleteData("disconnect-bot", "", "Disconnect the bot account (only system admins can do this)")
	disconnectBot.RoleID = model.SystemAdminRoleId
	cmd.AddCommand(disconnectBot)

	return cmd
}

func (p *Plugin) ExecuteCommand(_ *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	split := strings.Fields(args.Command)
	command := split[0]
	var parameters []string
	action := ""
	if len(split) > 1 {
		action = split[1]
	}
	if len(split) > 2 {
		parameters = split[2:]
	}

	if command != "/"+msteamsCommand {
		return &model.CommandResponse{}, nil
	}

	if action == "link" {
		return p.executeLinkCommand(args, parameters)
	}

	if action == "unlink" {
		return p.executeUnlinkCommand(args)
	}

	if action == "show" {
		return p.executeShowCommand(args)
	}

	if action == "connect" {
		return p.executeConnectCommand(args)
	}

	if action == "connect-bot" {
		return p.executeConnectBotCommand(args)
	}

	if action == "disconnect" {
		return p.executeDisconnectCommand(args)
	}

	if action == "disconnect-bot" {
		return p.executeDisconnectBotCommand(args)
	}

	return p.cmdError(args.UserId, args.ChannelId, "Unknown command. Valid options: link, unlink and show.")
}

func (p *Plugin) executeLinkCommand(args *model.CommandArgs, parameters []string) (*model.CommandResponse, *model.AppError) {
	channel, appErr := p.API.GetChannel(args.ChannelId)
	if appErr != nil {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to get the current channel information.")
	}

	if channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup {
		return p.cmdError(args.UserId, args.ChannelId, "Linking/unlinking a direct or group message is not allowed")
	}

	canLinkChannel := p.API.HasPermissionToChannel(args.UserId, args.ChannelId, model.PermissionManageChannelRoles)
	if !canLinkChannel {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to link the channel. You have to be a channel admin to link it.")
	}

	if len(parameters) < 2 {
		return p.cmdError(args.UserId, args.ChannelId, "Invalid link command, please pass the MS Teams team id and channel id as parameters.")
	}

	if !p.store.CheckEnabledTeamByTeamID(args.TeamId) {
		return p.cmdError(args.UserId, args.ChannelId, "This team is not enabled for MS Teams sync.")
	}

	link, err := p.store.GetLinkByChannelID(args.ChannelId)
	if err == nil && link != nil {
		return p.cmdError(args.UserId, args.ChannelId, "A link for this channel already exists. Please unlink the channel before you link again with another channel.")
	}

	link, err = p.store.GetLinkByMSTeamsChannelID(parameters[0], parameters[1])
	if err == nil && link != nil {
		return p.cmdError(args.UserId, args.ChannelId, "A link for this channel already exists. Please unlink the channel before you link again with another channel.")
	}

	client, err := p.GetClientForUser(args.UserId)
	if err != nil {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to link the channel, looks like your account is not connected to MS Teams")
	}

	_, err = client.GetChannel(parameters[0], parameters[1])
	if err != nil {
		return p.cmdError(args.UserId, args.ChannelId, "MS Teams channel not found or you don't have the permissions to access it.")
	}

	err = p.store.StoreChannelLink(&storemodels.ChannelLink{
		MattermostTeam:    channel.TeamId,
		MattermostChannel: channel.Id,
		MSTeamsTeam:       parameters[0],
		MSTeamsChannel:    parameters[1],
		Creator:           args.UserId,
	})
	if err != nil {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to create new link.")
	}

	if p.getConfiguration().SyncUsers > 0 {
		msChannelMembers, msUsersMap := p.getMSTeamsUsersAndMap(client, parameters[0], parameters[1])
		mmUsers, mmUsersMap := p.getMattermostUsersAndMap(channel.Id)
		batchSize := 50

		go func() {
			wg := sync.WaitGroup{}
			p.addMattermostUser(&wg, args.TeamId, args.ChannelId, msChannelMembers, mmUsersMap, batchSize)
			p.addMSTeamsUser(&wg, client, parameters[0], parameters[1], mmUsers, msUsersMap, batchSize)
			wg.Wait()
		}()
	}

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, "The MS Teams channel is now linked to this Mattermost channel.")
	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeUnlinkCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	channel, appErr := p.API.GetChannel(args.ChannelId)
	if appErr != nil {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to get the current channel information.")
	}

	if channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup {
		return p.cmdError(args.UserId, args.ChannelId, "Linking/unlinking a direct or group message is not allowed")
	}

	canLinkChannel := p.API.HasPermissionToChannel(args.UserId, args.ChannelId, model.PermissionManageChannelRoles)
	if !canLinkChannel {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to unlink the channel, you have to be a channel admin to unlink it.")
	}

	if _, err := p.store.GetLinkByChannelID(channel.Id); err != nil {
		return p.cmdError(args.UserId, args.ChannelId, "This Mattermost channel is not linked to any MS Teams channel.")
	}

	if err := p.store.DeleteLinkByChannelID(channel.Id); err != nil {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to delete link.")
	}

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, "The MS Teams channel is no longer linked to this Mattermost channel.")
	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeShowCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	link, err := p.store.GetLinkByChannelID(args.ChannelId)
	if err != nil || link == nil {
		return p.cmdError(args.UserId, args.ChannelId, "Link doesn't exist.")
	}

	msteamsTeam, err := p.msteamsAppClient.GetTeam(link.MSTeamsTeam)
	if err != nil {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to get the MS Teams team information.")
	}

	msteamsChannel, err := p.msteamsAppClient.GetChannel(link.MSTeamsTeam, link.MSTeamsChannel)
	if err != nil {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to get the MS Teams channel information.")
	}

	text := fmt.Sprintf(
		"This channel is linked to the MS Teams Channel \"%s\" (with id: %s) in the Team \"%s\" (with the id: %s).",
		msteamsChannel.DisplayName,
		msteamsChannel.ID,
		msteamsTeam.DisplayName,
		msteamsTeam.ID,
	)

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, text)
	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeConnectCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	state, _ := json.Marshal(map[string]string{})

	codeVerifier := model.NewId()
	appErr := p.API.KVSet("_code_verifier_"+args.UserId, []byte(codeVerifier))
	if appErr != nil {
		return p.cmdError(args.UserId, args.ChannelId, "Error trying to connect the account, please, try again.")
	}

	connectURL := msteams.GetAuthURL(p.GetURL()+"/oauth-redirect", p.configuration.TenantID, p.configuration.ClientID, p.configuration.ClientSecret, string(state), codeVerifier)
	p.sendBotEphemeralPost(args.UserId, args.ChannelId, fmt.Sprintf("Visit the URL for the auth dialog: %v", connectURL))
	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeConnectBotCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	if !p.API.HasPermissionTo(args.UserId, model.PermissionManageSystem) {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to connect the bot account, only system admins can connect the bot account.")
	}

	state, _ := json.Marshal(map[string]string{"auth_type": "bot"})

	codeVerifier := model.NewId()
	appErr := p.API.KVSet("_code_verifier_"+p.GetBotUserID(), []byte(codeVerifier))
	if appErr != nil {
		return p.cmdError(args.UserId, args.ChannelId, "Error trying to connect the bot account, please, try again.")
	}

	connectURL := msteams.GetAuthURL(p.GetURL()+"/oauth-redirect", p.configuration.TenantID, p.configuration.ClientID, p.configuration.ClientSecret, string(state), codeVerifier)

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, fmt.Sprintf("Visit the URL for the auth dialog: %v", connectURL))
	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeDisconnectCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	teamsUserID, err := p.store.MattermostToTeamsUserID(args.UserId)
	if err != nil {
		p.sendBotEphemeralPost(args.UserId, args.ChannelId, "Error: the account is not connected")
		return &model.CommandResponse{}, nil
	}

	if _, err = p.store.GetTokenForMattermostUser(args.UserId); err != nil {
		p.sendBotEphemeralPost(args.UserId, args.ChannelId, "Error: the account is not connected")
		return &model.CommandResponse{}, nil
	}

	err = p.store.SetUserInfo(args.UserId, teamsUserID, nil)
	if err != nil {
		p.sendBotEphemeralPost(args.UserId, args.ChannelId, fmt.Sprintf("Error: unable to disconnect your account, %s", err.Error()))
		return &model.CommandResponse{}, nil
	}

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, "Your account has been disconnected.")

	return &model.CommandResponse{}, nil
}

func (p *Plugin) executeDisconnectBotCommand(args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	if !p.API.HasPermissionTo(args.UserId, model.PermissionManageSystem) {
		return p.cmdError(args.UserId, args.ChannelId, "Unable to connect the bot account, only system admins can connect the bot account.")
	}

	botTeamsUserID, err := p.store.MattermostToTeamsUserID(p.userID)
	if err != nil {
		p.sendBotEphemeralPost(args.UserId, args.ChannelId, "Error: unable to find the connected bot account")
		return &model.CommandResponse{}, nil
	}
	err = p.store.SetUserInfo(p.userID, botTeamsUserID, nil)
	if err != nil {
		p.sendBotEphemeralPost(args.UserId, args.ChannelId, fmt.Sprintf("Error: unable to disconnect the bot account, %s", err.Error()))
		return &model.CommandResponse{}, nil
	}

	p.sendBotEphemeralPost(args.UserId, args.ChannelId, "The bot account has been disconnected.")

	return &model.CommandResponse{}, nil
}

func getAutocompletePath(path string) string {
	return "plugins/" + pluginID + "/autocomplete/" + path
}

func (p *Plugin) getMSTeamsUsersAndMap(client msteams.Client, teamID, channeID string) ([]msteams.User, map[string]msteams.User) {
	msChannelMembers, err := client.ListChannelMembers(teamID, channeID)
	if err != nil {
		p.API.LogError("Unable to get MS Teams users to add in the channel", "error", err)
		return nil, nil
	}

	msUsersMap := make(map[string]msteams.User, len(msChannelMembers))
	for _, u := range msChannelMembers {
		msUsersMap[u.Mail] = u
	}

	return msChannelMembers, msUsersMap
}

func (p *Plugin) getMattermostUsersAndMap(channelID string) ([]*model.User, map[string]*model.User) {
	mmChannelMembers, getErr := p.API.GetChannelMembers(channelID, 0, math.MaxInt32)
	if getErr != nil {
		p.API.LogError("Unable to get Mattermost users to add in the channel", "error", getErr)
		return nil, nil
	}

	var mmUsers []*model.User
	mmUsersMap := make(map[string]*model.User, len(mmChannelMembers))
	for _, u := range mmChannelMembers {
		user, err := p.API.GetUser(u.UserId)
		if err != nil {
			p.API.LogError("Unable to get the user", "error", err)
			continue
		}

		mmUsers = append(mmUsers, user)
		mmUsersMap[user.Email] = user
	}

	return mmUsers, mmUsersMap
}

func (p *Plugin) addMattermostUser(wg *sync.WaitGroup, teamID, channelID string, msChannelMembers []msteams.User, mmUsersMap map[string]*model.User, batchSize int) {
	for i := 0; i < (len(msChannelMembers)/batchSize)+1; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := i * batchSize; j < getMin((i*batchSize)+batchSize, len(msChannelMembers)); j++ {
				u := msChannelMembers[j]
				_, ok := mmUsersMap[u.Mail]
				if !ok {
					mmUserID, err := p.store.TeamsToMattermostUserID(u.ID)
					if err != nil {
						userUUID := uuid.Parse(u.ID)
						encoding := base32.NewEncoding("ybndrfg8ejkmcpqxot1uwisza345h769").WithPadding(base32.NoPadding)
						shortUserID := encoding.EncodeToString(userUUID)
						username := slug.Make(u.DisplayName) + "_" + u.ID

						newUser, appErr := p.API.CreateUser(&model.User{
							Password:  utils.GenerateRandomPassword(),
							Email:     u.Mail,
							RemoteId:  &shortUserID,
							FirstName: u.DisplayName,
							Username:  username,
						})

						if appErr != nil {
							p.API.LogError("Unable to create new user", "error", appErr)
							continue
						}

						setErr := p.store.SetUserInfo(newUser.Id, u.ID, nil)
						if setErr != nil {
							p.API.LogError("Unable to store the user", "error", setErr)
							continue
						}
					}

					if _, err := p.API.GetTeamMember(teamID, mmUserID); err != nil {
						if _, addErr := p.API.CreateTeamMember(teamID, mmUserID); addErr != nil {
							p.API.LogError("Unable to add user to the team", "error", addErr)
							continue
						}
					}

					if _, err := p.API.AddChannelMember(channelID, mmUserID); err != nil {
						p.API.LogError("Unable to add user to the channel", "error", err)
					}
				}
			}
		}(i)
	}
}

func (p *Plugin) addMSTeamsUser(wg *sync.WaitGroup, client msteams.Client, teamID, channelID string, mmUsers []*model.User, msUsersMap map[string]msteams.User, batchSize int) {
	for i := 0; i < (len(mmUsers)/batchSize)+1; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := i * batchSize; j < getMin((i*batchSize)+batchSize, len(mmUsers)); j++ {
				u := mmUsers[j]
				_, ok := msUsersMap[u.Email]
				if !ok {
					msTeamsUserID, err := p.store.MattermostToTeamsUserID(u.Id)
					if err != nil {
						p.API.LogError("Unable to get teams user ID", "error", err)
						continue
					}

					if err := client.AddChannelMember(teamID, channelID, msTeamsUserID); err != nil {
						p.API.LogError("Unable to add user to the channel", "error", err)
					}
				}
			}
		}(i)
	}
}

func getMin(a, b int) int {
	if a < b {
		return a
	}

	return b
}
