import {useDispatch} from 'react-redux';

import {alertSeverity} from 'constants/common.constants';
import {showAlert as showAlertAction} from 'reducers/snackbar';
import {SnackbarColor} from 'components/Snackbar/Snackbar.types';

const useAlert = () => {
    const dispatch = useDispatch();

    /**
	 * Show snackbar on RHs
	 * @param payload Alert message and severity
	 */
    const showAlert = ({
        message,
        severity = alertSeverity.default,
    }: {
        severity?: SnackbarColor;
        message: string;
    }) => {
        dispatch(
            showAlertAction({
                message,
                severity,
            }),
        );
    };

    return showAlert;
};

export default useAlert;
