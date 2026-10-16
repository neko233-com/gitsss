export function initAdminSidebar(): void {
  for (const link of document.querySelectorAll<HTMLAnchorElement>('.admin-sidebar-menu .item.active')) link.setAttribute('aria-current', 'page');
  for (const group of document.querySelectorAll<HTMLDetailsElement>('.admin-sidebar-menu > details')) {
    group.open = group.querySelector('.item.active') !== null;
  }
}
