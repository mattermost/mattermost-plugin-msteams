import Client from 'client';
import manifest from 'manifest';

export default function getUserSettings(serverRoute: string, disabled: boolean) {
    return {
        id: manifest.id,
        icon: `${serverRoute}/plugins/${manifest.id}/public/icon.svg`,
        uiName: manifest.name,
        action: disabled ? {
            title: 'Connect your Microsoft Teams Account',
            text: 'Connect your Mattermost and Microsoft Teams accounts to receive notifications in Mattermost for chats and group chats when you\'re away or offline in Microsoft Teams.',
            buttonText: 'Connect account',
            onClick: () => window.open(`${Client.url}/connect?from_preferences=true`),
        } : undefined, //eslint-disable-line no-undefined
        sections: [makeNotificationsSetting(disabled)],
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
                    text: 'Enabled',
                    value: 'on',
                    helpText: 'The MS Teams bot will notify you in Mattermost for chats and group chats when you\'re away or offline in Microsoft Teams.',
                },
                {
                    text: 'Disabled',
                    value: 'off',
                    helpText: 'You won’t be notified in Mattermost.',
                },
            ],
            type: 'radio' as const,
            default: 'off',
        }],
    };
}
