import React from 'react';

import {Dialog as MMDialog, LinearProgress, DialogProps} from '@brightscout/mattermost-ui-library';

export const Dialog = ({
    children,
    isLoading = false,
    ...rest
}: DialogProps & {isLoading?: boolean, children: React.ReactNode}) => (
    <MMDialog
        {...rest}
    >
        {isLoading && <LinearProgress className='absolute w-full left-0 top-62'/>}
        {children}
    </MMDialog>
);
