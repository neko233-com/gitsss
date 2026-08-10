import {audioMimeTypeForExtension, inspectGameAssetMetadata} from './frontend-game-assets.ts';

describe('game asset preview metadata', () => {
  test('maps common game audio formats to browser media types', () => {
    expect(audioMimeTypeForExtension('wav')).toBe('audio/wav');
    expect(audioMimeTypeForExtension('flac')).toBe('audio/flac');
    expect(audioMimeTypeForExtension('opus')).toContain('audio/ogg');
    expect(audioMimeTypeForExtension('png')).toBe('');
  });

  test('reads KTX2 dimensions and level metadata without a decoder', () => {
    const bytes = new Uint8Array(68);
    bytes.set([0xab, 0x4b, 0x54, 0x58, 0x20, 0x32, 0x30, 0xbb, 0x0d, 0x0a, 0x1a, 0x0a], 0);
    const view = new DataView(bytes.buffer);
    view.setUint32(20, 1024, true);
    view.setUint32(24, 512, true);
    view.setUint32(40, 10, true);
    view.setUint32(44, 2, true);

    expect(inspectGameAssetMetadata('ktx2', bytes)).toContain('KTX2 1024x512');
    expect(inspectGameAssetMetadata('ktx2', bytes)).toContain('10 mip levels');
  });
});
