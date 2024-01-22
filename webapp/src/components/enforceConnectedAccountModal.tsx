import React, {useCallback, useState, useEffect} from 'react';

import Client from '../client';
import {id as pluginId} from '../manifest';

import './enforceConnectedAccountModal.css';

export default function EnforceConnectedAccountModal() {
    const [open, setOpen] = useState(false);
    const [canSkip, setCanSkip] = useState(false);
    const [connecting, setConnecting] = useState(false);
    const iconURL = `/plugins/${pluginId}/public/msteams-sync-icon.svg`;

    const skip = useCallback(() => {
        setOpen(false);
    }, []);

    const connectAccount = useCallback(() => {
        Client.connect().then((result) => {
            setConnecting(true);
            window.open(result?.connectUrl, '_blank');
        });
    }, []);

    useEffect(() => {
        Client.needsConnect().then((result) => {
            setOpen(result.needsConnect);
            setCanSkip(result.canSkip);
        });
    }, []);

    const checkConnected = useCallback(async () => {
        const result = await Client.needsConnect();
        if (!result.needsConnect) {
            setOpen(false);
            setConnecting(false);
        }
    }, []);

    useEffect(() => {
        let interval: any = 0;
        if (connecting) {
            interval = setInterval(checkConnected, 1000);
        }
        return () => {
            if (interval) {
                clearInterval(interval);
            }
        };
    }, [connecting]);

    if (!open) {
        return null;
    }

    return (
        <div className='EnforceConnectedAccountModal'>
            <img src={iconURL}/>
            <h1>{'Connect your Microsoft Teams Account'}</h1>
            {!connecting && <p>{'This server requires you to connect your Mattermost account with your Microsoft Teams account.'}</p>}
            {!connecting && (
                <button
                    className='btn btn-primary'
                    onClick={connectAccount}
                >
                    {'Connect account'}
                </button>
            )}
            {connecting && <p className='connectUrl'>{'Please go to the new window and complete the login process'}</p>}
            {canSkip && !connecting && (
                <a
                    className='skipLink'
                    onClick={skip}
                >
                    {'Skip for now'}
                </a>
            )}
        </div>
    );
}
