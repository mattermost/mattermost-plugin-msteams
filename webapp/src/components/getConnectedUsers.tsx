import React from 'react';

type Props = {
    label: string;
    disabled: boolean;
};

const GettConnectedUsers = ({label}: Props) => {
    const handleClick = () => {
        window.location.href = '/plugins/com.mattermost.msteams-sync/connected-users';
    };

    return (
        <div
            style={styles.divMargin}
        >
            <p>
                {'Download a report of all Mattermost users connected to MS Teams'}
            </p>
            <button
                className='btn btn-primary'
                style={styles.buttonMargin}
                onClick={handleClick}
            >
                {label}
            </button>
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

export default GettConnectedUsers;
