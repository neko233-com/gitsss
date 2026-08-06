import type {FrontendRenderFunc} from '../plugin.ts';
import {basename} from '../../utils.ts';

const MAX_PIXELS = 50_000_000;

function read16(bytes: Uint8Array, offset: number): number {
  return bytes[offset] | (bytes[offset + 1] << 8);
}

function read32(bytes: Uint8Array, offset: number): number {
  return (bytes[offset] | (bytes[offset + 1] << 8) | (bytes[offset + 2] << 16) | (bytes[offset + 3] << 24)) >>> 0;
}

function writePixel(target: Uint8ClampedArray, index: number, r: number, g: number, b: number, a = 255): void {
  target[index] = r;
  target[index + 1] = g;
  target[index + 2] = b;
  target[index + 3] = a;
}

function tgaPixels(bytes: Uint8Array, width: number, height: number, pixelSize: number, imageType: number, descriptor: number): Uint8ClampedArray | null {
  if (bytes[1] !== 0 || (imageType !== 2 && imageType !== 10) || (pixelSize !== 24 && pixelSize !== 32)) return null;
  const output = new Uint8ClampedArray(width * height * 4);
  let sourceOffset = 18 + bytes[0];
  let pixelIndex = 0;
  let lastColor: [number, number, number, number] = [0, 0, 0, 255];
  const writeSourcePixelAt = (index: number, r: number, g: number, b: number, a: number) => {
    const sourceX = index % width;
    const sourceY = Math.floor(index / width);
    const x = descriptor & 0x10 ? width - sourceX - 1 : sourceX;
    const y = descriptor & 0x20 ? sourceY : height - sourceY - 1;
    writePixel(output, (y * width + x) * 4, r, g, b, a);
  };
  const writeSourcePixel = () => {
    if (sourceOffset + pixelSize / 8 > bytes.length || pixelIndex >= width * height) return false;
    const b = bytes[sourceOffset++];
    const g = bytes[sourceOffset++];
    const r = bytes[sourceOffset++];
    const a = pixelSize === 32 ? bytes[sourceOffset++] : 255;
    lastColor = [r, g, b, a];
    writeSourcePixelAt(pixelIndex, r, g, b, a);
    pixelIndex++;
    return true;
  };

  while (pixelIndex < width * height) {
    if (imageType === 10) {
      if (sourceOffset >= bytes.length) return null;
      const packet = bytes[sourceOffset++];
      const count = (packet & 0x7f) + 1;
      if (packet & 0x80) {
        const start = sourceOffset;
        if (!writeSourcePixel()) return null;
        for (let i = 1; i < count; i++) {
          if (pixelIndex >= width * height) return null;
          writeSourcePixelAt(pixelIndex, lastColor[0], lastColor[1], lastColor[2], lastColor[3]);
          pixelIndex++;
        }
        if (sourceOffset === start) return null;
      } else {
        for (let i = 0; i < count; i++) if (!writeSourcePixel()) return null;
      }
    } else if (!writeSourcePixel()) {
      return null;
    }
  }
  return output;
}

function ddsColor(value: number): [number, number, number] {
  return [((value >> 11) & 0x1f) * 255 / 31, ((value >> 5) & 0x3f) * 255 / 63, (value & 0x1f) * 255 / 31];
}

function ddsPixels(bytes: Uint8Array, width: number, height: number, fourCC: string): Uint8ClampedArray | null {
  if (fourCC !== 'DXT1' && fourCC !== 'DXT3' && fourCC !== 'DXT5') return null;
  const blockBytes = fourCC === 'DXT1' ? 8 : 16;
  const output = new Uint8ClampedArray(width * height * 4);
  let sourceOffset = 128;
  for (let blockY = 0; blockY < height; blockY += 4) {
    for (let blockX = 0; blockX < width; blockX += 4) {
      if (sourceOffset + blockBytes > bytes.length) return null;
      const color0 = read16(bytes, sourceOffset);
      const color1 = read16(bytes, sourceOffset + 2);
      const colors = [ddsColor(color0), ddsColor(color1), [0, 0, 0], [0, 0, 0]];
      if (color0 > color1 || fourCC !== 'DXT1') {
        colors[2] = [((colors[0][0] * 2 + colors[1][0]) / 3), ((colors[0][1] * 2 + colors[1][1]) / 3), ((colors[0][2] * 2 + colors[1][2]) / 3)];
        colors[3] = [((colors[0][0] + colors[1][0] * 2) / 3), ((colors[0][1] + colors[1][1] * 2) / 3), ((colors[0][2] + colors[1][2] * 2) / 3)];
      } else {
        colors[2] = [((colors[0][0] + colors[1][0]) / 2), ((colors[0][1] + colors[1][1]) / 2), ((colors[0][2] + colors[1][2]) / 2)];
      }
      const colorBits = read32(bytes, sourceOffset + 4);
      for (let py = 0; py < 4; py++) {
        for (let px = 0; px < 4; px++) {
          const x = blockX + px;
          const y = blockY + py;
          const colorIndex = (colorBits >> (2 * (py * 4 + px))) & 3;
          if (x >= width || y >= height) continue;
          const alpha = fourCC === 'DXT1' && color0 <= color1 && colorIndex === 3 ? 0 : 255;
          writePixel(output, (y * width + x) * 4, colors[colorIndex][0], colors[colorIndex][1], colors[colorIndex][2], alpha);
        }
      }
      sourceOffset += 8;
      if (fourCC === 'DXT3') {
        for (let row = 0; row < 4; row++) {
          const alphaBits = read16(bytes, sourceOffset + row * 2);
          for (let column = 0; column < 4; column++) {
            const x = blockX + column;
            const y = blockY + row;
            if (x < width && y < height) output[(y * width + x) * 4 + 3] = ((alphaBits >> (column * 4)) & 0xf) * 17;
          }
        }
        sourceOffset += 8;
      } else if (fourCC === 'DXT5') {
        const alpha0 = bytes[sourceOffset];
        const alpha1 = bytes[sourceOffset + 1];
        const alphaValues = [alpha0, alpha1, 0, 0, 0, 0, 0, 0];
        if (alpha0 > alpha1) {
          for (let i = 2; i < 8; i++) alphaValues[i] = (alpha0 * (8 - i) + alpha1 * (i - 1)) / 7;
        } else {
          for (let i = 2; i < 6; i++) alphaValues[i] = (alpha0 * (6 - i) + alpha1 * (i - 1)) / 5;
          alphaValues[6] = 0;
          alphaValues[7] = 255;
        }
        let alphaBits = 0n;
        for (let i = 0; i < 6; i++) alphaBits |= BigInt(bytes[sourceOffset + 2 + i]) << BigInt(i * 8);
        for (let i = 0; i < 16; i++) {
          const x = blockX + i % 4;
          const y = blockY + Math.floor(i / 4);
          if (x < width && y < height) output[(y * width + x) * 4 + 3] = alphaValues[Number((alphaBits >> BigInt(i * 3)) & 7n)];
        }
        sourceOffset += 8;
      }
    }
  }
  return output;
}

function rawLink(): string {
  return window.location.href.replace('/render/', '/raw/');
}

function renderNotice(container: HTMLElement, filename: string, message: string): void {
  container.classList.add('game-asset-preview');
  const card = document.createElement('div');
  card.className = 'game-asset-preview-card';
  const title = document.createElement('strong');
  title.textContent = filename;
  const description = document.createElement('p');
  description.textContent = message;
  const link = document.createElement('a');
  link.href = rawLink();
  link.target = '_blank';
  link.rel = 'nofollow';
  link.textContent = 'Open raw asset';
  card.append(title, description, link);
  container.append(card);
}

export const frontendRender: FrontendRenderFunc = async (opts): Promise<boolean> => {
  const filename = basename(opts.treePath);
  const extension = filename.toLowerCase().split('.').pop() || '';
  const bytes = opts.contentBytes();
  let width = 0;
  let height = 0;
  let pixels: Uint8ClampedArray | null = null;
  if (extension === 'tga' && bytes.length >= 18) {
    width = read16(bytes, 12);
    height = read16(bytes, 14);
    if (width * height <= MAX_PIXELS) pixels = tgaPixels(bytes, width, height, bytes[16], bytes[2], bytes[17]);
  } else if (extension === 'dds' && bytes.length >= 128 && String.fromCharCode(...bytes.slice(0, 4)) === 'DDS ') {
    width = read32(bytes, 16);
    height = read32(bytes, 12);
    const fourCC = String.fromCharCode(...bytes.slice(84, 88));
    if (width * height <= MAX_PIXELS) pixels = ddsPixels(bytes, width, height, fourCC);
  }
  if (pixels && width > 0 && height > 0) {
    const canvas = document.createElement('canvas');
    canvas.width = width;
    canvas.height = height;
    canvas.className = 'game-asset-preview-canvas';
    canvas.getContext('2d')!.putImageData(new ImageData(pixels, width, height), 0, 0);
    opts.container.classList.add('game-asset-preview');
    opts.container.append(canvas);
    return true;
  }
  renderNotice(opts.container, filename, 'This game texture format is indexed safely. Use raw/source view for the original asset when a browser decoder is unavailable.');
  return true;
};
