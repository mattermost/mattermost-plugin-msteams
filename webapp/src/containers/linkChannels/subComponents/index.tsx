import React from 'react';

import {Modal, AutoComplete} from '@brightscout/mattermost-ui-library';

import './styles.scss';

type LinkChannelsProps = {
    open: boolean;
    close: () => void;
}

const items = [
    {
        label: 'Team 1',
        value: 'Team 1',
    },
    {
        label: 'Team 2',
        value: 'Team 2',
    },
    {
        label: 'Team 3',
        value: 'Team 3',
    },
    {
        label: 'Team 4',
        value: 'Team 4',
    },
    {
        label: 'Team 5',
        value: 'Team 5',
    },
]

const LinkChannelsModal = ({open, close}: LinkChannelsProps) => {
    console.log('11', open);
    return (
        <Modal
            show={open}
            onCloseHandler={close}
            title='Select a Mattermost Team'
            subtitle='Select the Mattermost Team that contains the channel you are going to link'
            className='link-channels-modal'
        >
            <>
               <AutoComplete
                    fullWidth={true}
                    items={items}
                    label='Search Teams'
               />
            </>
        </Modal>
    );
};

export default LinkChannelsModal;
