// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

type Props = {
    label: string;
    disabled: boolean;
};

const GetConnectedUsersSetting = ({label}: Props) => {
    return (
        <div
            style={styles.divMargin}
        >
            <p>
                {'Download a report of all Mattermost users connected to MS Teams'}
            </p>
            <a
                href='/plugins/com.mattermost.msteams-sync/connected-users/download'
                className='btn btn-primary'
                rel='noreferrer'
                target='_self'
                download={true}
            >
                {label}
            </a>
        </div>
    );
};

const styles = {
    divMargin: {
        marginTop: '20px',
    },
    buttonMargin: {
        marginTop: '8px',
    },
};

export default GetConnectedUsersSetting;
