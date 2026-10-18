import {stripTags} from '../utils.ts';
import {hideElem, queryElemChildren, showElem} from '../utils/dom.ts';
import {POST} from '../modules/fetch.ts';
import {showErrorToast, type Toast} from '../modules/toast.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';

const {appSubUrl} = window.config;

export function initRepoTopicBar() {
  const mgrBtn = document.querySelector<HTMLButtonElement>('#manage_topic');
  if (!mgrBtn) return;

  const editDiv = document.querySelector('#topic_edit')!;
  const viewDiv = document.querySelector('#repo-topics')!;
  const topicDropdown = editDiv.querySelector<HTMLElement>('.ui.dropdown')!;
  const errorText = editDiv.querySelector<HTMLElement>('#repo-topic-error')!;
  const saveButton = editDiv.querySelector<HTMLButtonElement>('#save_topic')!;
  const cancelButton = editDiv.querySelector<HTMLButtonElement>('#cancel_topic_edit')!;
  let savedTopics = editDiv.querySelector<HTMLInputElement>('input[name=topics]')!.value.split(',').filter(Boolean);
  let saving = false;
  let lastErrorToast: Toast | null = null;

  mgrBtn.addEventListener('click', () => {
    hideElem([viewDiv, mgrBtn]);
    showElem(editDiv);
    errorText.hidden = true;
    topicDropdown.querySelector<HTMLInputElement>('input.search')!.focus();
  });

  document.querySelector('#cancel_topic_edit')!.addEventListener('click', () => {
    lastErrorToast?.hideToast();
    hideElem(editDiv);
    fomanticQuery(topicDropdown).dropdown('set exactly', savedTopics);
    showElem([viewDiv, mgrBtn]);
    mgrBtn.focus();
  });

  saveButton.addEventListener('click', async () => {
    if (saving) return;
    saving = true; saveButton.disabled = true; cancelButton.disabled = true; errorText.hidden = true;
    lastErrorToast?.hideToast();
    const topics = editDiv.querySelector<HTMLInputElement>('input[name=topics]')!.value;

    const data = new FormData();
    data.append('topics', topics);

    try {
      const response = await POST(saveButton.getAttribute('data-link')!, {data});

      if (response.ok) {
        const responseData = await response.json();
        if (responseData.status === 'ok') {
          savedTopics = responseData.topics || topics.split(',').filter(Boolean);
          queryElemChildren(viewDiv, '.repo-topic', (el) => el.remove());
          if (topics.length) {
            const topicArray: string[] = Array.from(savedTopics);
            topicArray.sort();
            for (const topic of topicArray) {
              // TODO: sort items in topicDropdown, or items in edit div will have different order to the items in view div
              // !!!! it SHOULD and MUST match the code in "home_sidebar_top.tmpl" !!!!
              const link = document.createElement('a');
              link.classList.add('repo-topic', 'ui', 'large', 'label', 'gt-ellipsis');
              link.href = `${appSubUrl}/explore/repos?q=${encodeURIComponent(topic)}&topic=1`;
              link.textContent = topic;
              viewDiv.append(link);
            }
          }
          hideElem(editDiv);
          fomanticQuery(topicDropdown).dropdown('set exactly', savedTopics);
          showElem([viewDiv, mgrBtn]);
          mgrBtn.focus();
        }
      } else if (response.status === 422) {
        // how to test: input topic like " invalid topic " (with spaces), and select it from the list, then "Save"
        const responseData = await response.json();
        errorText.textContent = responseData.message; errorText.hidden = false;
        lastErrorToast = showErrorToast(responseData.message, {duration: 5000});
        if (responseData.invalidTopics && responseData.invalidTopics.length > 0) {
          const {invalidTopics} = responseData;
          const topicLabels = queryElemChildren(topicDropdown, 'a.ui.label');
          for (const [index, value] of topics.split(',').entries()) {
            if (invalidTopics.includes(value)) {
              topicLabels[index].classList.remove('green');
              topicLabels[index].classList.add('red');
            }
          }
        }
      } else {throw new Error(editDiv.getAttribute('data-error')!)}
    } catch {
      errorText.textContent = editDiv.getAttribute('data-error')!; errorText.hidden = false;
    } finally {
      saving = false; saveButton.disabled = false; cancelButton.disabled = false;
    }
  });

  fomanticQuery(topicDropdown).dropdown({
    allowAdditions: true,
    maxSelections: 25,
    forceSelection: false,
    fullTextSearch: 'exact',
    fields: {name: 'description', value: 'data-value'},
    saveRemoteData: false,
    label: {
      transition: 'horizontal flip',
      duration: 200,
      variation: false,
    },
    apiSettings: {
      url: `${appSubUrl}/explore/topics/search?q={query}`,
      throttle: 500,
      cache: false,
      onResponse(this: any, res: any) {
        const formattedResponse = {
          success: false,
          results: [] as Array<Record<string, any>>,
        };
        const query = stripTags(this.urlData.query.trim());
        let found_query = false;
        const current_topics = Array.from(queryElemChildren(topicDropdown, 'a.ui.label.visible'), (el) => el.getAttribute('data-value'));

        if (res.topics) {
          let found = false;
          for (const {topic_name} of res.topics) {
            // skip currently added tags
            if (current_topics.includes(topic_name)) {
              continue;
            }

            if (topic_name.toLowerCase() === query.toLowerCase()) {
              found_query = true;
            }
            formattedResponse.results.push({description: topic_name, 'data-value': topic_name});
            found = true;
          }
          formattedResponse.success = found;
        }

        if (query.length > 0 && !found_query) {
          formattedResponse.success = true;
          formattedResponse.results.unshift({description: query, 'data-value': query});
        } else if (query.length > 0 && found_query) {
          formattedResponse.results.sort((a, b) => {
            if (a.description.toLowerCase() === query.toLowerCase()) return -1;
            if (b.description.toLowerCase() === query.toLowerCase()) return 1;
            if (a.description > b.description) return -1;
            if (a.description < b.description) return 1;
            return 0;
          });
        }

        return formattedResponse;
      },
    },
    onLabelCreate(value: string) {
      value = value.toLowerCase().trim();
      this.attr('data-value', value).contents().first().replaceWith(document.createTextNode(value));
      return fomanticQuery(this);
    },
    onAdd(addedValue: string, _addedText: any, $addedChoice: any) {
      addedValue = addedValue.toLowerCase().trim();
      $addedChoice[0].setAttribute('data-value', addedValue);
      $addedChoice[0].setAttribute('data-text', addedValue);
    },
  });
  topicDropdown.addEventListener('keydown', (event: KeyboardEvent) => {
    if (event.key !== 'Enter' || event.isComposing) return;
    // Enter selects a tag even while remote suggestions are pending; it must not quick-save the form.
    event.stopPropagation();
    const input = topicDropdown.querySelector<HTMLInputElement>('input.search')!;
    const value = stripTags(input.value).trim().toLowerCase();
    if (value) {
      event.preventDefault();
      fomanticQuery(topicDropdown).dropdown('set selected', value);
      input.value = '';
    }
  });
  topicDropdown.querySelector<HTMLInputElement>('input.search')!.setAttribute('aria-label', mgrBtn.textContent.trim());
}
