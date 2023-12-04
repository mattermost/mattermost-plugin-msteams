export type IconName = 'user' | 'message' | 'connectAccount' | 'warning' | 'close'

export type IconProps = {
    iconName: IconName;
    height?: number;
    width?: number;
    className?: string;
}
