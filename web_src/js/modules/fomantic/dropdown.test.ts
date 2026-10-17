import '../../../fomantic/build/fomantic.js';
import {createElementFromHTML} from '../../utils/dom.ts';
import {initAriaDropdownPatch, hideScopedEmptyDividers} from './dropdown.ts';

test('dropdown-item-literal-text', () => {
  // a "choice" workflow_dispatch input can offer the string "false" as an option.
  // jQuery `.data()` would coerce `data-text="false"` to the boolean `false`, which then renders as empty text.
  const $dropdown = $(`<select class="ui dropdown">
    <option value="1">1</option>
    <option value="0">0</option>
    <option value="true">true</option>
    <option value="false">false</option>
  </select>`).dropdown();
  for (const value of ['1', '0', 'true', 'false']) {
    $dropdown.dropdown('set selected', value);
    expect($dropdown.dropdown('get text')).toEqual(value);
    expect($dropdown.dropdown('get value')).toEqual(value);
  }
});

test('hideScopedEmptyDividers-simple', () => {
  const container = createElementFromHTML(`<div>
<div class="divider"></div>
<div class="item">a</div>
<div class="divider"></div>
<div class="divider"></div>
<div class="divider"></div>
<div class="item">b</div>
<div class="divider"></div>
</div>`);
  hideScopedEmptyDividers(container);
  expect(container.innerHTML).toEqual(`
<div class="divider hidden"></div>
<div class="item">a</div>
<div class="divider hidden"></div>
<div class="divider hidden"></div>
<div class="divider"></div>
<div class="item">b</div>
<div class="divider hidden"></div>
`);
});

test('hideScopedEmptyDividers-items-all-filtered', () => {
  const container = createElementFromHTML(`<div>
<div class="any"></div>
<div class="divider"></div>
<div class="item filtered">a</div>
<div class="item filtered">b</div>
<div class="divider"></div>
<div class="any"></div>
</div>`);
  hideScopedEmptyDividers(container);
  expect(container.innerHTML).toEqual(`
<div class="any"></div>
<div class="divider hidden"></div>
<div class="item filtered">a</div>
<div class="item filtered">b</div>
<div class="divider"></div>
<div class="any"></div>
`);
});

test('hideScopedEmptyDividers-hide-last', () => {
  const container = createElementFromHTML(`<div>
<div class="item">a</div>
<div class="divider" data-scope="b"></div>
<div class="item tw-hidden" data-scope="b">b</div>
</div>`);
  hideScopedEmptyDividers(container);
  expect(container.innerHTML).toEqual(`
<div class="item">a</div>
<div class="divider hidden" data-scope="b"></div>
<div class="item tw-hidden" data-scope="b">b</div>
`);
});

test('hideScopedEmptyDividers-scoped-items', () => {
  const container = createElementFromHTML(`<div>
<div class="item" data-scope="">a</div>
<div class="divider" data-scope="b"></div>
<div class="item tw-hidden" data-scope="b">b</div>
<div class="divider" data-scope=""></div>
<div class="item" data-scope="">c</div>
</div>`);
  hideScopedEmptyDividers(container);
  expect(container.innerHTML).toEqual(`
<div class="item" data-scope="">a</div>
<div class="divider hidden" data-scope="b"></div>
<div class="item tw-hidden" data-scope="b">b</div>
<div class="divider hidden" data-scope=""></div>
<div class="item" data-scope="">c</div>
`);
});

test('dropdown popup owns menu semantics and generated selectors reuse visible labels', () => {
  $.fn.fomanticExt ||= {};
  initAriaDropdownPatch();
  const fixture = createElementFromHTML('<div><div class="field"><label>Language</label><div class="ui selection dropdown"><input type="hidden" name="language"><span class="text">English</span><div class="menu"><div class="item" data-value="en">English</div></div></div></div><div class="ui dropdown"><span class="text">More actions</span><div class="menu"><a class="item" href="#one">One</a></div></div></div>');
  document.body.append(fixture);
  $(fixture).find('.ui.dropdown').dropdown();
  const trigger = fixture.querySelectorAll('.ui.dropdown')[1];
  expect(trigger.getAttribute('role')).toBe('button');
  expect(trigger.getAttribute('aria-haspopup')).toBe('menu');
  expect(trigger.querySelector('.menu')!.getAttribute('role')).toBe('menu');
  const combo = fixture.querySelector('[role="combobox"]')!;
  expect(document.querySelector(`#${combo.getAttribute('aria-labelledby')!}`)!.textContent).toBe('Language');
  fixture.remove();
});
