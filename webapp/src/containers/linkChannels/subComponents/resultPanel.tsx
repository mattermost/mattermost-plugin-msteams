import React from 'react';

import Constants from 'src/constants';

type ResultPanelProps = {
    className: string;
    errorMessage?: string;
}

const ResultPanel = ({
    className = '',
    errorMessage,
}: ResultPanelProps): JSX.Element => {
    return (
        <div className={`result-panel ${className}`}>
            <span className='result-panel__icon'>
                {errorMessage ? (
                    <img src={Constants.errorIconUrl}/>
                ) : (
                    <img src={Constants.checkIconUrl}/>
                )}
            </span>
            <div className='result-panel__body'>
                <span className='title'>
                    {errorMessage ? 'Error' : 'Congratulations'}
                </span>
                <p className='message'>
                    {errorMessage || 'Your channels have been successfully linked.'}
                </p>
            </div>
        </div>
    );
};

export default ResultPanel;
