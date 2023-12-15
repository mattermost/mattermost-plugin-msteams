import React from 'react';

import {Dialog as MMDialog, LinearProgress, DialogProps} from '@brightscout/mattermost-ui-library';

export const Dialog = ({
    show,
    title,
    destructive,
    primaryButtonText,
    secondaryButtonText,
    onSubmitHandler,
    onCloseHandler,
    children,
    isLoading,
    className,
}: DialogProps & {isLoading?: boolean, children: React.ReactNode}) => (
    <MMDialog
        show={show}
        destructive={destructive}
        primaryButtonText={primaryButtonText}
        secondaryButtonText={secondaryButtonText}
        onCloseHandler={onCloseHandler}
        onSubmitHandler={onSubmitHandler}
        className={className}
        title={title}
    >
        {isLoading && <LinearProgress className='absolute w-full left-0 top-62'/>}
        {children}
    </MMDialog>
);
