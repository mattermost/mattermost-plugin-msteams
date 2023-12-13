import {ReduxState} from 'types/common/store.d';

import 'isomorphic-fetch';

import {mockTestState} from './mockState';

const mockDispatch = jest.fn();
let mockState: ReduxState;

jest.mock('react-redux', () => ({
    ...jest.requireActual('react-redux') as typeof import('react-redux'),
    useSelector: (selector: (state: typeof mockState) => unknown) => selector(mockState),
    useDispatch: () => mockDispatch,
}));

beforeAll(() => {
    mockState = mockTestState as unknown as ReduxState;
});
