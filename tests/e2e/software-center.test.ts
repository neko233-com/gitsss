import {expect, test} from '@playwright/test';
import {apiCreateOrg, apiDeleteOrg, login, randomString} from './utils.ts';

test('filters and safely organizes software with a stale preview refresh', async ({page, request}, testInfo) => {
  const orgName = `software-${randomString(8)}`;
  await Promise.all([apiCreateOrg(request, orgName), login(page)]);

  try {
    await page.goto(`/${orgName}/-/software`);
    await page.setViewportSize({width: 1366, height: 768});
    const rows = page.locator('[data-software-rows]');
    await page.locator('[data-software-upload]').setInputFiles([
      {name: 'windows.msi', mimeType: 'application/octet-stream', buffer: Buffer.from('windows')},
      {name: 'macos.dmg', mimeType: 'application/octet-stream', buffer: Buffer.from('macos')},
      {name: 'ios.ipa', mimeType: 'application/octet-stream', buffer: Buffer.from('ios')},
      {name: 'android.apk', mimeType: 'application/octet-stream', buffer: Buffer.from('android')},
      {name: 'portable.zip', mimeType: 'application/zip', buffer: Buffer.from('portable')},
      {name: 'notes.txt', mimeType: 'text/plain', buffer: Buffer.from('keep in place')},
    ]);
    await expect(rows.locator('tr')).toHaveCount(6, {timeout: 30_000});

    await page.locator('[data-software-platform]').selectOption('Windows');
    await expect(page).toHaveURL(/platform=Windows/);
    await expect(rows).toContainText('windows.msi');
    await expect(rows.locator('tr')).toHaveCount(1);

    await page.locator('[data-software-platform]').selectOption('');
    await expect(rows.locator('tr')).toHaveCount(6);
    await page.locator('[data-software-organize]').click();
    const dialog = page.locator('[data-software-organize-dialog]');
    await expect(dialog).toBeVisible();
    await expect(dialog.locator('[data-software-organize-change-count]')).toHaveText('5');
    await expect(dialog.locator('[data-software-organize-skipped-count]')).toHaveText('1');
    await page.screenshot({path: testInfo.outputPath('software-center-organize-preview.png'), fullPage: true});

    await using concurrentPage = await page.context().newPage();
    await concurrentPage.goto(`/${orgName}/-/software`);
    await concurrentPage.locator('[data-software-upload]').setInputFiles({
      name: 'windows-patch.exe', mimeType: 'application/octet-stream', buffer: Buffer.from('patch'),
    });
    await expect(concurrentPage.locator('[data-software-rows]')).toContainText('windows-patch.exe', {timeout: 30_000});

    await dialog.locator('[data-software-organize-apply]').click();
    await expect(dialog.locator('[data-software-organize-change-count]')).toHaveText('6');
    await expect(dialog.locator('[data-software-organize-apply]')).toBeEnabled();
    await dialog.locator('[data-software-organize-apply]').click();
    await expect(dialog).toBeHidden();
    await expect(rows).toContainText('Windows');
    await expect(rows).toContainText('macOS');
    await expect(rows).toContainText('iOS');
    await expect(rows).toContainText('Android');
    await expect(rows).toContainText('全平台');
    await expect(rows).toContainText('notes.txt');
    await expect(rows.locator('tr')).toHaveCount(6);
    await page.setViewportSize({width: 390, height: 844});
    await expect(page.locator('[data-software-rows]')).toBeVisible();
    await page.screenshot({path: testInfo.outputPath('software-center-mobile.png'), fullPage: true});
  } finally {
    await apiDeleteOrg(request, orgName);
  }
});
