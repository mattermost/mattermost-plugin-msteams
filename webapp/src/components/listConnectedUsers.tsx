import React from 'react';

type Props = {
    label: string;
    disabled: boolean;
};

const ListConnectedUsers = ({label}: Props) => {
    const handleClick = () => {
        window.location.href = '/plugins/com.mattermost.msteams-sync/list-connected-users';
    };

    return (
        <div
            style={{
                marginTop: '20px',
            }}
        >
            <p>
                {'Download a report of all Mattermost users connected to MS Teams'}
            </p>
            <button
                className='btn btn-primary'
                style={{
                    marginTop: '8px',
                }}
                onClick={handleClick}
            >
                {label}
            </button>
        </div>
    );
};

export default ListConnectedUsers;
