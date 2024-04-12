import React, {ChangeEvent, useRef, useState} from 'react';

import Client from 'client';

type Props = {
    label: string;
    disabled: boolean;
};

const InviteWhitelistSetting = ({label, disabled}: Props) => {
    const fileInputRef = useRef<HTMLInputElement>(null);
    const [pendingFile, setPendingFile] = useState<File>();
    const [statusMsg, setStatusMsg] = useState('');

    const onChange = (e: ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0];

        if (file) {
            setPendingFile(file);
            setStatusMsg('');
        }

        if (fileInputRef.current) {
            fileInputRef.current.value = '';
        }
    };

    const doUpload = async (e: React.MouseEvent<HTMLButtonElement>) => {
        e.preventDefault();

        if (!pendingFile) {
            return;
        }

        setStatusMsg('Uploading');

        try {
            await Client.uploadWhitelist(pendingFile);
            setStatusMsg('Upload successful - whitelist updated');
        } catch (err: any) {
            if (err.message) {
                setStatusMsg(err.message);
            } else {
                setStatusMsg('error while uploading');
            }
        }

        // eslint-disable-next-line no-undefined
        setPendingFile(undefined);
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
                    {statusMsg || pendingFile?.name}
                </div>
                <p className='help-text'>
                    {'Upload a CSV file containing mattermost user-emails that may receive invites to connect to MS Teams. NOTE: This will replace the entire whitelist, not add to it.'}
                </p>
                <div style={styles.divMargin}>
                    <a
                        href='/plugins/com.mattermost.msteams-sync/whitelist/download'
                        className='btn btn-primary btn-sm'
                        rel='noreferrer'
                        target='_self'
                        download={true}
                    >
                        {'Download Whitelist'}
                    </a>

                </div>

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
