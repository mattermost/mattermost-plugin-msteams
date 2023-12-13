/**
 * For a detailed explanation regarding each configuration property, visit:
 * https://jestjs.io/docs/configuration
 */

import type {Config} from 'jest';

const config: Config = {

    // Automatically clear mock calls, instances, contexts and results before every test
    clearMocks: true,

    // Indicates whether the coverage information should be collected while executing the test
    collectCoverage: true,

    // An array of glob patterns indicating a set of files for which coverage information should be collected
    collectCoverageFrom: [
        'src/components/**/*.{jsx,tsx}',
        'src/reducers/**/*.{js,jsx,ts,tsx}',
        'src/containers/**/*.{js,jsx,ts,tsx}',
    ],

    // The directory where Jest should output its coverage files
    coverageDirectory: 'coverage',

    // An array of directory names to be searched recursively up from the requiring module's location
    moduleDirectories: [
        'node_modules',
    ],

    // An array of file extensions your modules use
    // moduleFileExtensions: [
    //   "js",
    //   "mjs",
    //   "cjs",
    //   "jsx",
    //   "ts",
    //   "tsx",
    //   "json",
    //   "node"
    // ],
    moduleFileExtensions: ['ts', 'tsx', 'js', 'jsx'],

    // A map from regular expressions to module names or to arrays of module names that allow to stub out resources with a single module
    moduleNameMapper: {
        '^.+\\.(jpg|jpeg|png|gif|eot|otf|webp|svg|ttf|woff|woff2|mp4|webm|wav|mp3|m4a|aac|oga)$': 'identity-obj-proxy',
        '^.+\\.(css|less|scss)$': 'identity-obj-proxy',
        '^src/(.*)': '<rootDir>/src/$1',
        '^components/(.*)': '<rootDir>/src/components/$1',
        '^constants/(.*)': '<rootDir>/src/constants/$1',
        '^components': '<rootDir>/src/components/$1',
        '^utils': '<rootDir>/src/utils/$1',
        '^selectors': '<rootDir>/src/selectors/$1',
        '^hooks/(.*)': '<rootDir>/src/hooks/$1',
        '^reducers/(.*)': '<rootDir>/src/reducers/$1',
        '^services': '<rootDir>/src/services/$1',
        '^tests/(.*)': '<rootDir>/src/tests/$1',
    },

    // A preset that is used as a base for Jest's configuration
    preset: 'ts-jest',

    // A list of paths to directories that Jest should use to search for files in
    // roots: [
    //   "<rootDir>"
    // ],
    roots: ['<rootDir>/src/'],

    // A list of paths to modules that run some code to configure or set up the testing framework before each test
    // setupFilesAfterEnv: [],
    setupFilesAfterEnv: [
        '@testing-library/jest-dom',
        '<rootDir>/src/tests/setup.ts',
    ],

    // The test environment that will be used for testing
    testEnvironment: 'jsdom',

    // A map from regular expressions to paths to transformers
    transform: {
        '\\.[jt]sx?$': 'babel-jest',
    },

    // An array of regexp pattern strings that are matched against all source file paths, matched files will skip transformation
    // transformIgnorePatterns: [
    //   "/node_modules/",
    //   "\\.pnp\\.[^\\/]+$"
    // ],
    transformIgnorePatterns: [
        'node_modules/(?!(react-native|react-router|@brightscout)/)',
    ],
};

export default config;
