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
                href='/plugins/com.mattermost.msteams-sync/connected-users-file'
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
