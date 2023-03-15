import React, {useCallback, useState, useEffect} from 'react';

import Client from '../client';

import './enforceConnectedAccountModal.css';

const msteamsLogo = "https://upload.wikimedia.org/wikipedia/commons/c/c9/Microsoft_Office_Teams_%282018%E2%80%93present%29.svg"

export default function EnforceConnectedAccountModal() {
    const [open, setOpen] = useState(false)
    const [canSkip, setCanSkip] = useState(false)
    const [connectMessage, setConnectMessage] = useState('')

    const skip = useCallback(() => {
        setOpen(false)
    }, [])

    const connectAccount = useCallback(() => {
        Client.connect().then((result) => {
            setConnectMessage(result.message);
        })
    }, [])

    useEffect(() => {
        Client.needsConnect().then((result) => {
            setOpen(result.needsConnect)
            setCanSkip(result.canSkip)
        })
    }, [])

    useEffect(() => {
        let interval = 0
        if (connectMessage) {
            setInterval(() => {
                Client.needsConnect().then((result) => {
                    if (!result.needsConnect) {
                        setOpen(false)
                        setConnectMessage('')
                    }
                })
            }, 1000)
        }
        return () => {
            if (interval) {
                clearInterval(interval)
            }
        }
    }, [connectMessage])

    if (!open) {
        return null
    }

    return (
        <div className='EnforceConnectedAccountModal'>
            <img src={msteamsLogo}/>
            <h1>Connect your Microsoft Teams Account</h1>
            {!connectMessage && <p>For using this server you need to connect your mattermost account with your MS Teams account, to procced just click in the button</p>}
            {!connectMessage && <button className='btn btn-primary' onClick={connectAccount}>{'Connect account'}</button>}
            {connectMessage && <p className='connectMessage'>{connectMessage}</p>}
            {canSkip && !connectMessage && <a className="skipLink" onClick={skip}>{'Skip for now'}</a>}
        </div>
    )
}
