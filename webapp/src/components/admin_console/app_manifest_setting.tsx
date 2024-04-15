// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

type Props = {
    label: string;
    disabled: boolean;
};

export default function MSTeamsAppManifestSetting(props: Props) {
    return (
        <div>
            <p>
                {'To embed Mattermost within Microsoft Teams, an application manifest can be downloaded and installed as a MS Teams app. '}
                {'Clicking the Download button below will generate an application manifest that will embed this instance of Mattermost. '}
            </p>
            <p>
                {'Mattermost embedded in MS Teams can be used together with MSTeams Sync, or independently.'}
            </p>
            <a
                href='/plugins/com.mattermost.msteams-sync/iframe-manifest'
                className='btn btn-primary'
                rel='noreferrer'
                target='_self'
                style={styles.buttonBorder}
                download={true}
            >
                {props.label}
            </a>
        </div>
    );
}

const styles = {
    buttonBorder: {
        marginTop: '8px',
    },
};
