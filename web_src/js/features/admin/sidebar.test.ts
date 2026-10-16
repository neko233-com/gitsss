import {initAdminSidebar} from './sidebar.ts';

test('opens the current administration section while other sections remain expandable', () => {
  document.body.innerHTML = '<nav class="admin-sidebar-menu"><details open><summary>Maintenance</summary><a class="item">Dashboard</a></details><details open><summary>Integrations</summary><a class="item active">Webhooks</a></details></nav>';
  initAdminSidebar();
  const groups = document.querySelectorAll<HTMLDetailsElement>('details');
  expect(groups[0].open).toBe(false);
  expect(groups[1].open).toBe(true);
  expect(document.querySelector('.item.active')!.getAttribute('aria-current')).toBe('page');
  groups[0].open = true;
  expect(groups[0].open).toBe(true);
});
