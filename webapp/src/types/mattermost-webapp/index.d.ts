interface PluginRegistry {
    registerPostTypeComponent(typeName: string, component: React.ElementType)
    registerRootComponent(component: React.ElementType)
    registerAdminConsoleCustomSetting(key: string, component: React.ElementType)
    registerRightHandSidebarComponent(component:null | (() => JSX.Element), title: string | JSX.Element)
    registerChannelHeaderButtonAction(icon: JSX.Element | null, action: () => void, dropdownText: string | null, tooltipText: string | null)
    registerAppBarComponent(iconUrl: string, action: () => void, tooltipText: string)
    registerReducer(reducer)
    registerWebSocketEventHandler(event: string, handler: (msg: WebsocketEventParams) => void)
    unregisterComponent(id: string)

    // Add more if needed from https://developers.mattermost.com/extend/plugins/webapp/reference
}
