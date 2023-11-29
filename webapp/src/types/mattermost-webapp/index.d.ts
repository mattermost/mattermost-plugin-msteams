export type BasePluginConfigurationSetting = {
    name: string;
    title?: string;
    helpText?: string;
    default?: string;
}

export type PluginConfigurationRadioSettingOption = {
    value: string;
    text: string;
    helpText?: string;
}

export type PluginConfigurationRadioSetting = BasePluginConfigurationSetting & {
    type: 'radio';
    default: string;
    options: PluginConfigurationRadioSettingOption[];
}

export type PluginConfigurationSetting = PluginConfigurationRadioSetting

export type PluginConfigurationSection = {
    settings: PluginConfigurationSetting[];
    title: string;
    onSubmit?: (changes: Record<string, string>) => void;
}

export type PluginConfiguration = {
    id: string;
    uiName: string;
    icon?: string;
    sections: PluginConfigurationSection[];
}

export interface PluginRegistry {
    registerPostTypeComponent(typeName: string, component: React.ElementType)
    registerRootComponent(component: React.ElementType)
    registerAdminConsoleCustomSetting(key: string, component: React.ElementType)
    registerUserSettings?(setting: PluginConfiguration)

    // Add more if needed from https://developers.mattermost.com/extend/plugins/webapp/reference
}
