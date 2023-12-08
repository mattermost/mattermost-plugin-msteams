import React from 'react';
import {useDispatch} from 'react-redux';

import {Dialog as MMDialog, LinearProgress, DialogProps} from '@brightscout/mattermost-ui-library';

import usePluginApi from 'hooks/usePluginApi';
import {getDialogState} from 'selectors';
import {closeDialog} from 'reducers/dialog';

export const Dialog = ({onCloseHandler, onSubmitHandler}: Pick<DialogProps, 'onCloseHandler' | 'onSubmitHandler'>) => {
    const dispatch = useDispatch();
    const {state} = usePluginApi();
    const {show, title, description, destructive, primaryButtonText, secondaryButtonText, isLoading} = getDialogState(state);

    const handleClose = () => dispatch(closeDialog());

    return (
        <MMDialog
            description={description}
            destructive={destructive}
            show={show}
            primaryButtonText={primaryButtonText}
            secondaryButtonText={secondaryButtonText}
            onCloseHandler={() => {
                handleClose();
                onCloseHandler();
            }}
            onSubmitHandler={onSubmitHandler}
            className='disconnect-dialog'
            title={title}
        >
            {isLoading && <LinearProgress/>}
        </MMDialog>
    );
};
