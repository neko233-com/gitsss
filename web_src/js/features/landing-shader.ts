// The original branch-field fragment uses Paper Shaders only for the WebGL lifecycle.
const branchField = `#version 300 es
precision highp float;
uniform vec2 u_resolution;
uniform float u_time;
uniform vec3 u_background;
uniform vec3 u_ink;
out vec4 fragColor;
float branch(float x, float spread) { return .51 + spread * smoothstep(.08,.82,x) + .018*sin(x*7.+spread*3.); }
void main() {
 vec2 uv = gl_FragCoord.xy/u_resolution;
 float fade = smoothstep(.0,.14,uv.x)*smoothstep(1.,.90,uv.x)*smoothstep(.0,.12,uv.y)*smoothstep(1.,.90,uv.y);
 vec3 color = u_background;
 float dots = 0.; float lines = 0.; float nodes = 0.; float rings = 0.;
 for(int b=0;b<4;b++) {
  float spread = (float(b)-1.5)*.21;
  float y = branch(uv.x,spread);
  for(int n=-5;n<=5;n++) {
   float offset=float(n)*.010*(.3+uv.x);
   float dy=abs(uv.y-y-offset);
   vec2 cell=vec2(fract(uv.x*80.+float(n)*.23)-.5,(uv.y-y-offset)*100.);
   dots += (1.-smoothstep(.10,.22,length(cell)))*(.14+.2*(1.-abs(float(n))/6.));
   if(n==0) lines += (1.-smoothstep(.0008,.0022,dy))*.5;
  }
  for(int point=0;point<2;point++) {
   float phase=fract(.18+float(b)*.19+float(point)*.47+u_time*.014);
   vec2 center=vec2(.3+phase*.63,branch(.3+phase*.63,spread));
   vec2 delta=(uv-center)*vec2(u_resolution.x/u_resolution.y,1.);
   float d=length(delta);
   nodes += 1.-smoothstep(.012,.016,d);
   rings += (1.-smoothstep(.001,.003,abs(d-.019)))*.95;
   float spine=(1.-smoothstep(.001,.002,abs(delta.x)))*(1.-smoothstep(.12,.20,abs(delta.y)));
   dots+=spine*step(.55,fract(uv.y*70.))*.55;
  }
 }
 float field=clamp((dots+lines)*fade,0.,.8);
 color=mix(color,u_ink,field);
 color=mix(color,u_ink,clamp(nodes,0.,1.)*fade);
 color=mix(color,u_background,clamp(rings,0.,1.)*fade);
 fragColor=vec4(color,1.);
}`;

export async function initLandingShader(): Promise<void> {
  const target = document.querySelector<HTMLElement>('[data-landing-shader]');
  if (!target) return;
  const motion = document.querySelector<HTMLButtonElement>('[data-landing-motion]')!;
  const reduced = window.matchMedia('(prefers-reduced-motion: reduce)');
  const colors = getComputedStyle(target);
  const rgb = (value: string) => {
    const parsed = value.match(/[\d.]+/g)!.slice(0, 3).map((part) => Number(part) / 255);
    return parsed;
  };
  try {
    const {ShaderMount} = await import('../modules/paper-shader.ts');
    const background = getComputedStyle(document.body).backgroundColor;
    const shader = new ShaderMount(target, branchField, {u_background: rgb(background), u_ink: rgb(colors.color)}, {alpha: false, antialias: false, powerPreference: 'low-power'}, reduced.matches ? 0 : 0.35, 18000, 1, 298000);
    if (!target.querySelector('canvas')) throw new Error('WebGL unavailable');
    target.setAttribute('data-ready', 'true');
    target.setAttribute('data-shader-status', 'ready');
    motion.hidden = false;
    let playing = !reduced.matches;
    const update = () => {shader.setSpeed(playing ? 0.35 : 0); motion.textContent = motion.getAttribute(playing ? 'data-pause' : 'data-play'); motion.setAttribute('aria-pressed', String(playing))};
    motion.addEventListener('click', () => {playing = !playing; update()});
    reduced.addEventListener('change', () => {playing = !reduced.matches; update()});
    target.querySelector('canvas')!.addEventListener('webglcontextlost', (event) => {event.preventDefault(); shader.dispose(); target.setAttribute('data-shader-status', 'fallback'); target.removeAttribute('data-ready'); motion.hidden = true}, {once: true});
    window.addEventListener('pagehide', () => shader.dispose(), {once: true});
    update();
  } catch {
    target.setAttribute('data-shader-status', 'fallback');
    target.replaceChildren();
    target.removeAttribute('data-ready');
    motion.hidden = true;
  }
}
