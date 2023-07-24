import React, {useCallback, useState, useEffect} from 'react';

import usePluginApi from '../hooks/usePluginApi';
import useApiRequestCompletionState from '../hooks/useApiRequestCompletionState';

import Constants from '../constants';

import './enforceConnectedAccountModal.css';

export default function EnforceConnectedAccountModal() {
    const [open, setOpen] = useState(false);
    const [canSkip, setCanSkip] = useState(false);
    const [connecting, setConnecting] = useState(false);
    const [isInterval, setIsInterval] = useState(false);
    const {makeApiRequestWithCompletionStatus, getApiState} = usePluginApi();

    const skip = useCallback(() => {
        setOpen(false);
    }, []);

    const connectAccount = useCallback(() => {
        makeApiRequestWithCompletionStatus(Constants.pluginApiServiceConfigs.connect.apiServiceName);
    }, []);

    const {data: needsConnectData} = getApiState(Constants.pluginApiServiceConfigs.needsConnect.apiServiceName);
    const {data: connectData} = getApiState(Constants.pluginApiServiceConfigs.connect.apiServiceName);

    useApiRequestCompletionState({
        serviceName: Constants.pluginApiServiceConfigs.needsConnect.apiServiceName,
        handleSuccess: () => {
            if (needsConnectData) {
                const data = needsConnectData as NeedsConnectData;
                if (!isInterval) {
                    setOpen(data.needsConnect);
                    setCanSkip(data.canSkip);
                } else if (!data.needsConnect) {
                    setOpen(false);
                    setConnecting(false);
                }
            }
        },
    });

    useApiRequestCompletionState({
        serviceName: Constants.pluginApiServiceConfigs.connect.apiServiceName,
        handleSuccess: () => {
            if (connectData) {
                setConnecting(true);
                window.open((connectData as ConnectData).connectUrl, '_blank');
            }
        },
    });

    const checkConnected = useCallback(() => {
        makeApiRequestWithCompletionStatus(Constants.pluginApiServiceConfigs.needsConnect.apiServiceName);
    }, []);

    useEffect(() => {
        let interval: any = 0;
        if (connecting) {
            setIsInterval(true);
            interval = setInterval(checkConnected, 1000);
        }
        return () => {
            if (interval) {
                setIsInterval(false);
                clearInterval(interval);
            }
        };
    }, [connecting]);

    if (!open) {
        return null;
    }

    return (
        <div className='EnforceConnectedAccountModal'>
            <img src={Constants.iconUrl}/>
            <h1>{'Connect your Microsoft Teams Account'}</h1>
            {!connecting && <p>{'For using this server you need to connect your Mattermost account with your MS Teams account, to procced just click in the button'}</p>}
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
