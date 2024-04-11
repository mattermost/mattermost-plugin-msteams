import React, {ChangeEvent, useRef, useState} from 'react';

type Props = {
    label: string;
    disabled: boolean;
};

const InviteWhitelistSetting = ({label}: Props) => {
    const fileInputRef = useRef<HTMLInputElement>(null);
    const [pendingFile, setPendingFile] = useState<File>();

    const onChange = (e: ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0];

        if (file) {
            setPendingFile(file);
        }
    };

    const doUpload = () => {

    };

    return (
        <div className='form-group'>
            <label className='control-label col-sm-4'>
                {label}
            </label>
            <div className='col-sm-8'>
                <div className='file__upload'>
                    <button
                        type='button'
                        className={'btn btn-tertiary'}
                    >
                        {'Choose file'}
                    </button>
                    <input
                        ref={fileInputRef}
                        type='file'
                        accept='.csv'
                        onChange={onChange}
                    />
                </div>
                <button
                    className={'btn btn-primary'}
                    id='uploadWhitelistCsv'
                    disabled={!pendingFile}
                    onClick={doUpload}
                >
                    {'Upload'}
                </button>

                <div className='help-text m-0'>
                    {pendingFile?.name}
                </div>

                <p className='help-text'>
                    {'Upload a CSV file containing mattermost user-emails that may receive invites to connect to MS Teams. NOTE: This will replace the entire whitelist, be sure it is complete before uploading.'}
                </p>

                <p className='help-text'>
                    {'To view the current whitelist: '}
                    <button
                        type='button'
                        className={'btn btn-primary btn-sm'}
                    >
                        {'Download Whitelist'}
                    </button>
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
