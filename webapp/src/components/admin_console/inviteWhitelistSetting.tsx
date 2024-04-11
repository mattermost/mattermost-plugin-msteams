import React from 'react';

type Props = {
    label: string;
    disabled: boolean;
};

const InviteWhitelistSetting = ({label}: Props) => {
    return (
        <div
            className='form-group'
        >
            <label className='control-label col-sm-4'>{label}</label>
            <div className='col-sm-8'>
                <p>
                    {'Upload a CSV file containing mattermost users that will receive invites to connect to MS Teams'}
                </p>
            </div>

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

export default InviteWhitelistSetting;
