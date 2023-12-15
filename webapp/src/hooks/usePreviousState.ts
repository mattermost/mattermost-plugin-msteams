import {useEffect, useRef} from 'react';

// usePreviousState stores the previous state for a current state
function usePreviousState(value: Record<string, string>) {
    const ref = useRef<Record<string, string>>();
    useEffect(() => {
        ref.current = value;
    }, [value]);
    return ref.current;
}

export default usePreviousState;
