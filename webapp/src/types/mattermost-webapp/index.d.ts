export interface PluginRegistry {
    registerPostTypeComponent(typeName: string, component: React.ElementType)
    registerRootComponent(component: React.ElementType)
    registerAdminConsoleCustomSetting(key: string, component: React.ElementType)

    // Add more if needed from https://developers.mattermost.com/extend/plugins/webapp/reference
}
