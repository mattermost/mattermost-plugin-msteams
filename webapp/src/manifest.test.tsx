import manifest from './manifest';

describe('manifest module', () => {
    test('Plugin manifest, id and version are defined', () => {
        expect(manifest).toBeDefined();
        expect(manifest.id).toBeDefined();
        expect(manifest.version).toBeDefined();
    });
});
