import {useEffect, useRef} from 'react';

const vertexShader = `
attribute vec2 a_position;
void main() {
  gl_Position = vec4(a_position, 0.0, 1.0);
}
`;

const fragmentShader = `
precision mediump float;
uniform vec2 u_resolution;
uniform float u_time;
void main() {
  vec2 uv = gl_FragCoord.xy / u_resolution.xy;
  float wave = sin(uv.x * 5.5 + u_time * 0.65) * 0.08;
  wave += sin(uv.y * 3.5 - u_time * 0.4) * 0.06;
  float band = smoothstep(0.15, 0.85, uv.y + wave);
  vec3 deep = vec3(0.02, 0.08, 0.16);
  vec3 blue = vec3(0.05, 0.38, 0.78);
  vec3 violet = vec3(0.36, 0.16, 0.62);
  vec3 color = mix(deep, blue, band);
  color = mix(color, violet, 0.28 + 0.18 * sin(u_time * 0.5 + uv.x * 6.0));
  gl_FragColor = vec4(color, 1.0);
}
`;

function compile(gl: WebGLRenderingContext, type: number, source: string): WebGLShader {
  const shader = gl.createShader(type);
  if (!shader) throw new Error('shader');
  gl.shaderSource(shader, source);
  gl.compileShader(shader);
  if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
    const log = gl.getShaderInfoLog(shader) || 'compile failed';
    gl.deleteShader(shader);
    throw new Error(log);
  }
  return shader;
}

export function Aurora() {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return undefined;
    const gl = canvas.getContext('webgl', {antialias: false, alpha: true});
    if (!gl) return undefined;
    const program = gl.createProgram();
    if (!program) return undefined;
    let frame = 0;
    let disposed = false;
    try {
      const vs = compile(gl, gl.VERTEX_SHADER, vertexShader);
      const fs = compile(gl, gl.FRAGMENT_SHADER, fragmentShader);
      gl.attachShader(program, vs);
      gl.attachShader(program, fs);
      gl.linkProgram(program);
      gl.deleteShader(vs);
      gl.deleteShader(fs);
      if (!gl.getProgramParameter(program, gl.LINK_STATUS)) return undefined;
      const buffer = gl.createBuffer();
      gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
      gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 1, -1, -1, 1, 1, 1]), gl.STATIC_DRAW);
      const position = gl.getAttribLocation(program, 'a_position');
      gl.enableVertexAttribArray(position);
      gl.vertexAttribPointer(position, 2, gl.FLOAT, false, 0, 0);
      const resolution = gl.getUniformLocation(program, 'u_resolution');
      const time = gl.getUniformLocation(program, 'u_time');
      const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
      const draw = (now: number) => {
        if (disposed) return;
        const width = canvas.clientWidth || 1;
        const height = canvas.clientHeight || 1;
        if (canvas.width !== width || canvas.height !== height) {
          canvas.width = width;
          canvas.height = height;
        }
        gl.viewport(0, 0, canvas.width, canvas.height);
        gl.useProgram(program);
        gl.uniform2f(resolution, canvas.width, canvas.height);
        gl.uniform1f(time, reduce ? 0 : now / 1000);
        gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
        if (!reduce) frame = window.requestAnimationFrame(draw);
      };
      frame = window.requestAnimationFrame(draw);
    } catch {
      return undefined;
    }
    return () => {
      disposed = true;
      window.cancelAnimationFrame(frame);
      gl.deleteProgram(program);
    };
  }, []);

  return <canvas ref={canvasRef} className="gitsss-react-shader" aria-hidden="true" />;
}
