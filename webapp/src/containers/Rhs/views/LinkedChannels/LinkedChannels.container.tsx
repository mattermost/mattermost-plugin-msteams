import React, {useCallback, useEffect, useMemo, useState} from 'react';
import InfiniteScroll from 'react-infinite-scroll-component';

import {Spinner} from '@brightscout/mattermost-ui-library';

import {WarningCard, LinkedChannelCard} from 'components';
import usePluginApi from 'hooks/usePluginApi';
import {pluginApiServiceConfigs} from 'constants/apiService.constant';

import {defaultPage, defaultPerPage} from 'constants/common.constants';

import {channelListTitle, noMoreChannelsText} from 'constants/linkedChannels.constants';

import {mockLinkedChannels} from './LinkedChannels.mock';

import './LinkedChannels.styles.scss';

export const LinkedChannels = () => {
    // TODO: Add Linked channel list
    const {makeApiRequestWithCompletionStatus} = usePluginApi();
    const [totalLinkedChannels, setTotalLinkedChannels] = useState<ChannelLinkData[]>([]);
    const [paginationQueryParams, setPaginationQueryParams] = useState<PaginationQueryParams>({
        page: defaultPage,
        per_page: defaultPerPage,
    });

    // TODO: Remove this part used for mocking API call for infinite scroll.
    const getChannels = () => new Promise<void>((res) => {
        setTimeout(() => res(), 2000);
    });

    useEffect(() => {
        getChannels().then(() => {
            const linkedChannels = mockLinkedChannels.slice((paginationQueryParams.page * paginationQueryParams.per_page), (paginationQueryParams.page + 1) * paginationQueryParams.per_page);
            setTotalLinkedChannels([...totalLinkedChannels, ...(linkedChannels as ChannelLinkData[])]);
        });
    }, [paginationQueryParams]);

    const connectAccount = useCallback(() => {
        makeApiRequestWithCompletionStatus(
            pluginApiServiceConfigs.connect.apiServiceName,
        );
    }, []);

    // Increase the page number by 1
    const handlePagination = () => {
        setPaginationQueryParams({...paginationQueryParams, page: paginationQueryParams.page + 1,
        });
    };

    const hasMoreLinkedChannels = useMemo<boolean>(() => (
        (totalLinkedChannels.length - (paginationQueryParams.page * defaultPerPage) === defaultPerPage)
    ), [totalLinkedChannels]);

    return (
        <div className='msteams-sync-utils'>
            <div className='msteams-sync-rhs flex-1 d-flex flex-column'>
                <div className='p-20 d-flex flex-column gap-20'>
                    <WarningCard
                        onConnect={connectAccount}
                    />
                </div>
                <h4 className='font-16 lh-24 my-0 p-20 wt-600'>{channelListTitle}</h4>
                <div
                    id='scrollableArea'
                    className='scroll-container flex-1-0-0'
                >
                    <InfiniteScroll
                        dataLength={totalLinkedChannels.length}
                        next={handlePagination}
                        hasMore={hasMoreLinkedChannels}
                        loader={<Spinner className='scroll-container__spinner'/>}
                        endMessage={
                            <p className='text-center'>
                                <b>{noMoreChannelsText}</b>
                            </p>
                        }
                        scrollableTarget='scrollableArea'
                    >
                        {totalLinkedChannels.map(({msTeamsChannelID, ...rest}) => (
                            <LinkedChannelCard
                                channelId={msTeamsChannelID}
                                key={msTeamsChannelID}
                                {...rest}
                            />
                        ))
                        }
                    </InfiniteScroll>
                </div>
            </div>
        </div>
    );
};
