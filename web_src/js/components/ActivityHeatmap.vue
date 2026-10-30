<script lang="ts" setup>
import {computed, nextTick, ref} from 'vue';
import {defaultHeatmapBounds, defaultHeatmapColors, heatmapData, localDateKey, migrateHeatmapColors, validHeatmapBounds} from '../features/heatmap-data.ts';
import type {HeatmapValue} from '../features/heatmap-data.ts';
import {localUserSettings} from '../modules/user-settings.ts';

const props = defineProps<{
  values: HeatmapValue[];
  locale: {
    textTotalContributions: string;
    heatMapLocale: {months: string[]; days: string[]; on: string; more: string; less: string};
    noDataText: string;
    tooltipUnit: string;
    labels: Record<string, string>;
  };
}>();
const preferenceKey = 'gitsss:heatmap-display:v1';
const bounds = ref([...defaultHeatmapBounds]);
const colors = ref([...defaultHeatmapColors]);
try {
  const saved = JSON.parse(localUserSettings.getString(preferenceKey, 'null'));
  if (saved && validHeatmapBounds(saved.bounds) && Array.isArray(saved.colors) && saved.colors.length === 5 && saved.colors.every((c: unknown) => typeof c === 'string' && /^#[a-f0-9]{6}$/i.test(c))) {
    bounds.value = saved.bounds; colors.value = migrateHeatmapColors(saved.colors);
  }
} catch {}
const draftBounds = ref([...bounds.value]);
const draftColors = ref([...colors.value]);
const error = ref('');
const preferences = ref<HTMLDetailsElement>();
const calendar = ref<HTMLElement>();
const now = new Date();
const grid = computed(() => heatmapData(props.values, now, bounds.value));
const selectedDate = new URLSearchParams(location.search).get('date');
const focusDate = ref(grid.value.weeks.flat().some((day) => day.visible && day.key === selectedDate) ? selectedDate! : localDateKey(now));
const number = (value: number) => value.toLocaleString();
const summary = computed(() => [
  ['total', grid.value.summary.total], ['recent7', grid.value.summary.recent7], ['recent30', grid.value.summary.recent30],
  ['active', grid.value.summary.active], ['peak', grid.value.summary.peak],
] as Array<[string, number]>);
const range = (i: number) => i === 4 ? `${bounds.value[i]}+` : bounds.value[i] === bounds.value[i + 1] - 1 ? String(bounds.value[i]) : `${bounds.value[i]}–${bounds.value[i + 1] - 1}`;
const dayLabel = (day: {key: string; count: number}) => `${day.key} · ${number(day.count)} ${props.locale.tooltipUnit}`;

function save(reset = false) {
  const next = reset ? [...defaultHeatmapBounds] : draftBounds.value.map(Number);
  const nextColors = reset ? [...defaultHeatmapColors] : draftColors.value;
  if (!validHeatmapBounds(next)) {error.value = props.locale.labels.invalid; return;}
  bounds.value = [...next]; colors.value = [...nextColors];
  draftBounds.value = [...next]; draftColors.value = [...nextColors]; error.value = '';
  localUserSettings.setString(preferenceKey, JSON.stringify({bounds: next, colors: nextColors}));
  preferences.value!.open = false;
}
function handleDayClick(date: string) {
  const params = new URLSearchParams(document.location.search);
  if (params.get('date') === date) params.delete('date'); else params.set('date', date);
  params.delete('page');
  window.location.search = params.toString() ? `?${params}` : '';
}
async function moveFocus(event: KeyboardEvent, key: string) {
  const offset = {ArrowUp: -1, ArrowDown: 1, ArrowLeft: -7, ArrowRight: 7}[event.key];
  if (offset === undefined && event.key !== 'Home' && event.key !== 'End') return;
  event.preventDefault();
  const days = grid.value.weeks.flat().filter((day) => day.visible);
  const index = days.findIndex((day) => day.key === key);
  const target = event.key === 'Home' ? 0 : event.key === 'End' ? days.length - 1 : Math.max(0, Math.min(days.length - 1, index + offset!));
  focusDate.value = days[target].key;
  await nextTick();
  calendar.value!.querySelector<HTMLButtonElement>(`[data-date="${focusDate.value}"]`)!.focus();
}
</script>
<template>
  <section class="heatmap-card" :aria-label="locale.labels.title">
    <dl class="heatmap-summary">
      <div v-for="[key, value] in summary" :key="key"><dt>{{ locale.labels[key] }}</dt><dd>{{ number(value) }}<small v-if="key === 'active'"> / 365</small></dd></div>
    </dl>
    <div class="heatmap-calendar-scroll">
      <div ref="calendar" class="heatmap-calendar" :style="{'--heatmap-weeks': grid.weeks.length}">
        <div class="heatmap-months"><span v-for="month in grid.months" :key="month.week" :style="{gridColumn: `${month.week + 1} / span 2`}">{{ locale.heatMapLocale.months[month.month] }}</span></div>
        <div class="heatmap-body">
          <div class="heatmap-day-labels"><span v-for="day in [1, 3, 5]" :key="day" :style="{gridRow: day + 1}">{{ locale.heatMapLocale.days[day] }}</span></div>
          <div class="heatmap-weeks">
            <div v-for="(week, w) in grid.weeks" :key="w" class="heatmap-week">
              <template v-for="day in week" :key="day.key">
                <button
                  v-if="day.visible" type="button" class="heatmap-day" :class="{'is-selected': day.key === selectedDate}"
                  :data-date="day.key" :data-count="day.count" :data-level="day.level"
                  :tabindex="day.key === focusDate ? 0 : -1" :title="dayLabel(day)" :aria-label="dayLabel(day)" :aria-pressed="day.key === selectedDate"
                  :style="{backgroundColor: day.level ? colors[day.level - 1] : 'var(--color-secondary-alpha-60)'}"
                  @focus="focusDate = day.key" @keydown="moveFocus($event, day.key)" @click="handleDayClick(day.key)"
                />
                <span v-else/>
              </template>
            </div>
          </div>
        </div>
      </div>
    </div>
    <div class="heatmap-footer">
      <p>{{ locale.labels.method }}</p>
      <div class="heatmap-legend" :aria-label="locale.labels.ranges">
        <span><i class="heatmap-swatch"/>0</span><span v-for="(color, i) in colors" :key="i"><i class="heatmap-swatch" :style="{backgroundColor: color}"/>{{ range(i) }}</span>
      </div>
    </div>
    <details ref="preferences" class="heatmap-preferences">
      <summary>{{ locale.labels.settings }}</summary>
      <form @submit.prevent="save()">
        <div class="heatmap-range-fields"><label v-for="(_, i) in draftBounds" :key="i"><span>{{ locale.labels.minimum }} {{ i + 1 }}</span><span class="heatmap-range-inputs"><input v-model.number="draftBounds[i]" type="number" :readonly="i === 0" min="1" max="1000000" step="1" required :aria-label="`${locale.labels.minimum} ${i + 1}`"><input v-model="draftColors[i]" type="color" :aria-label="`${locale.labels.color} ${i + 1}`"></span></label></div>
        <p v-if="error" class="text red" role="alert">{{ error }}</p><div class="heatmap-preference-actions"><button type="submit" class="ui primary compact button">{{ locale.labels.apply }}</button><button type="button" class="ui basic compact button" @click="save(true)">{{ locale.labels.reset }}</button><span>{{ locale.labels.local }}</span></div>
      </form>
    </details>
  </section>
</template>
