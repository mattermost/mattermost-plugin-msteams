import React, {useCallback} from 'react';
import {useDispatch} from 'react-redux';

import {DialogProps} from '@brightscout/mattermost-ui-library';

import {showDialog as showDialogComponent, closeDialog} from 'reducers/dialog';

import {Dialog} from 'components';
import {DialogState} from 'types/common/store.d';

const useDialog = ({onCloseHandler, onSubmitHandler}: Pick<DialogProps, 'onCloseHandler' | 'onSubmitHandler'>) => {
    const dispatch = useDispatch();

    const showDialog = (props: DialogState) => dispatch(showDialogComponent(props));

    const hideDialog = () => dispatch(closeDialog());

    const DialogComponent = useCallback(() => (
        <Dialog
            onCloseHandler={onCloseHandler}
            onSubmitHandler={onSubmitHandler}
        />
    ), []);

    return {
        DialogComponent,
        showDialog,
        hideDialog,
    };
};

export default useDialog;
