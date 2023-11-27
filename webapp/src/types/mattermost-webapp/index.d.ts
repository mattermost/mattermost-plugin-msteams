export interface PluginRegistry {
    registerPostTypeComponent(typeName: string, component: React.ElementType)
    registerRootComponent(component: React.ElementType)
    registerAdminConsoleCustomSetting(key: string, component: React.ElementType)
    registerUserSettings?(setting: PluginConfiguration)

    // Add more if needed from https://developers.mattermost.com/extend/plugins/webapp/reference
}

export type PluginConfiguration = {
    id: string;
    uiName: string;
    icon?: string;
    sections: PluginConfigurationSection[];
}

export type PluginConfigurationSection = {
    settings: PluginConfigurationSetting[];
    title: string;
    onSubmit?: (changes: {[name: string]: string}) => void;
}

export type BasePluginConfigurationSetting = {
    name: string;
    title?: string;
    helpText?: string;
    default?: string;
}

export type PluginConfigurationRadioSetting = BasePluginConfigurationSetting & {
    type: 'radio';
    default: string;
    options: PluginConfigurationRadioSettingOption[];
}

export type PluginConfigurationRadioSettingOption = {
    value: string;
    text: string;
    helpText?: string;
}

export type PluginConfigurationSetting = PluginConfigurationRadioSetting
