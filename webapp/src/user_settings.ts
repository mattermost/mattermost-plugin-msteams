import Client from 'client';
import manifest from 'manifest';

export default function getUserSettings(serverRoute: string, disabled: boolean) {
    return {
        id: manifest.id,
        icon: `${serverRoute}/plugins/${manifest.id}/public/icon.svg`,
        uiName: manifest.name,
        action: disabled ? {
            title: 'Connect your Microsoft Teams Account',
            text: 'Connect your Mattermost and Microsoft Teams accounts to get the ability to link and synchronise channel-based collaboration with Microsoft Teams.',
            buttonText: 'Connect account',
            onClick: () => window.open(`${Client.url}/connect?from_preferences=true`),
        } : undefined, //eslint-disable-line no-undefined
        sections: [makePlatformSetting(disabled), makeNotificationsSetting(disabled)],
    };
}

function makePlatformSetting(disabled: boolean) {
    return {
        settings: [{
            name: 'platform',
            options: [
                {
                    text: 'Mattermost',
                    value: 'mattermost',
                    helpText: 'You will get notifications in Mattermost for synced messages and channels. You will need to disable notifications in Microsoft Teams to avoid duplicates. **[Learn more](https://mattermost.com/pl/ms-teams-plugin-end-user-learn-more)**',
                },
                {
                    text: 'Microsoft Teams',
                    value: 'msteams',
                    helpText: 'Notifications in Mattermost will be muted for linked channels and DMs to prevent duplicates. You can unmute any linked channel or DM/GM if you wish to receive notifications. **[Learn more](https://mattermost.com/pl/ms-teams-plugin-end-user-learn-more)**',
                },
            ],
            type: 'radio' as const,
            default: 'mattermost',
            helpText: 'Note: Unread statuses for linked channels and DMs will not be synced between Mattermost & Microsoft Teams.',
        }],
        title: 'Primary platform for communication',
        disabled,
    };
}

function makeNotificationsSetting(disabled: boolean) {
    return {
        title: 'Notifications',
        disabled,
        settings: [{
            name: 'notifications',
            options: [
                {
                    text: 'Notifications from chats and group chats',
                    value: 'on',
                    helpText: 'You will get notified by the MS Teams bot whenever you receive a message from a chat or group chat in Teams.',
                },
                {
                    text: 'Disabled',
                    value: 'off',
                    helpText: 'You will not get notified.',
                },
            ],
            type: 'radio' as const,
            default: 'off',
        }],
    };
}
