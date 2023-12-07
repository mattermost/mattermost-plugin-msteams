import {SnackbarColor} from 'components/Snackbar/Snackbar.types';

export const pluginTitle = 'Microsoft Teams Sync';

export const siteUrl = 'SITEURL';

export const defaultPage = 0;

export const defaultPerPage = 20;

// Severity used in alert component
export const alertSeverity: Record<SnackbarColor, SnackbarColor> = {
    success: 'success',
    error: 'error',
    default: 'default',
} as const;

export const alertTimeout = 4000;
