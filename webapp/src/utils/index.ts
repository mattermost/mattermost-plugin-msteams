import {id as pluginId} from '../manifest';

const getBaseUrls = (siteURL?: string): {pluginApiBaseUrl: string} => {
    const pluginApiBaseUrl = `${siteURL}/plugins/${pluginId}`;
    return {pluginApiBaseUrl};
};

const getIconUrl = (iconName: string): string => `/plugins/${pluginId}/public/${iconName}`;

// Takes a userId and generates a link to that user's profile image
const getAvatarUrl = (userId: string, siteURL: string): string => {
    return `${getBaseUrls(siteURL).pluginApiBaseUrl}/avatar/${userId}`;
};

/**
 * Uses closure functionality to implement debouncing
 * @param {function} func Function on which debouncing is to be applied
 * @param {number} limit The time limit for debouncing, the minimum pause in function calls required for the function to be actually called
 * @returns {(args: Array<any>) => void} a function with debouncing functionality applied on it
 */
const debounce: (func: (args: Record<string, string>, type?: string) => void, limit: number) => (args: Record<string, string>) => void = (
    func: (args: Record<string, string>) => void,
    limit: number,
): (args: Record<string, string>) => void => {
    let timer: NodeJS.Timeout;

    /**
     * This is to use the functionality of closures so that timer isn't reinitialized once initialized
     * @param {Array<any>} args
     * @returns {void}
     */

    // eslint-disable-next-line func-names
    return function(args: Record<string, string>): void {
        clearTimeout(timer);
        timer = setTimeout(() => func({...args}), limit);
    };
};

export default {getBaseUrls, getIconUrl, getAvatarUrl, debounce};
