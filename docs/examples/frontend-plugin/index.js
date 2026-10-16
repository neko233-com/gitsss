export function mount(root) {
  const node = document.createElement('p');
  node.textContent = 'hello from a frontend plugin';
  root.append(node);
}
