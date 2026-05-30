<template>
  <AppLayout>
    <TablePageLayout>
      <template #actions>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
              {{ t('admin.affiliates.levels.title') }}
            </h1>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
              {{ t('admin.affiliates.levels.description') }}
            </p>
          </div>
          <div class="flex items-center gap-2">
            <button
              type="button"
              class="btn btn-secondary px-2 md:px-3"
              :disabled="loading"
              :title="t('common.refresh')"
              @click="loadLevels"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button
              type="button"
              class="btn btn-primary inline-flex items-center gap-2"
              @click="openCreateDialog"
            >
              <Icon name="plus" size="sm" />
              <span>{{ t('admin.affiliates.levels.add') }}</span>
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="levels"
          :loading="loading"
          row-key="id"
          default-sort-key="sort_order"
          default-sort-order="asc"
        >
          <template #cell-name="{ row }">
            <div class="space-y-0.5">
              <div class="flex items-center gap-2">
                <span class="font-medium text-gray-900 dark:text-white">{{ row.name }}</span>
                <span
                  v-if="row.is_default"
                  class="rounded bg-primary-50 px-1.5 py-0.5 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300"
                >
                  {{ t('admin.affiliates.levels.defaultBadge') }}
                </span>
              </div>
              <div class="font-mono text-xs text-gray-500 dark:text-dark-400">{{ row.code }}</div>
            </div>
          </template>
          <template #cell-rebate_rate_percent="{ row }">
            <span class="text-sm font-semibold text-emerald-600 dark:text-emerald-400">
              {{ formatPercent(row.rebate_rate_percent) }}
            </span>
          </template>
          <template #cell-threshold="{ row }">
            <div class="space-y-0.5 text-sm">
              <div>{{ t('admin.affiliates.levels.minInvited', { count: row.min_invited_count }) }}</div>
              <div class="text-gray-500 dark:text-dark-400">
                {{ t('admin.affiliates.levels.minHistory', { amount: formatAmount(row.min_history_quota) }) }}
              </div>
            </div>
          </template>
          <template #cell-enabled="{ row }">
            <span
              :class="[
                'inline-flex rounded px-2 py-1 text-xs font-medium',
                row.enabled
                  ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
                  : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-300',
              ]"
            >
              {{ row.enabled ? t('common.enabled') : t('common.disabled') }}
            </span>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex items-center gap-1">
              <button
                type="button"
                class="rounded p-2 text-gray-500 transition hover:bg-gray-100 hover:text-primary-600 dark:text-dark-300 dark:hover:bg-dark-700"
                :title="t('common.edit')"
                @click="openEditDialog(row)"
              >
                <Icon name="edit" size="sm" />
              </button>
              <button
                type="button"
                class="rounded p-2 text-gray-500 transition hover:bg-red-50 hover:text-red-600 dark:text-dark-300 dark:hover:bg-red-950/30"
                :title="t('admin.affiliates.levels.disable')"
                @click="confirmDisable(row)"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </template>
        </DataTable>
      </template>
    </TablePageLayout>

    <BaseDialog
      :show="dialogOpen"
      :title="editingLevel ? t('admin.affiliates.levels.editTitle') : t('admin.affiliates.levels.createTitle')"
      width="wide"
      @close="closeDialog"
    >
      <div class="grid gap-4 md:grid-cols-2">
        <div>
          <label class="input-label">{{ t('admin.affiliates.levels.code') }}</label>
          <input v-model="form.code" type="text" class="input font-mono uppercase" maxlength="32" placeholder="BRONZE" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.affiliates.levels.name') }}</label>
          <input v-model="form.name" type="text" class="input" :placeholder="t('admin.affiliates.levels.namePlaceholder')" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.affiliates.levels.rate') }}</label>
          <div class="relative">
            <input v-model.number="form.rebate_rate_percent" type="number" min="0" max="100" step="0.01" class="input pr-8" />
            <span class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-gray-400">%</span>
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('admin.affiliates.levels.sortOrder') }}</label>
          <input v-model.number="form.sort_order" type="number" step="1" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.affiliates.levels.minInvitedLabel') }}</label>
          <input v-model.number="form.min_invited_count" type="number" min="0" step="1" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.affiliates.levels.minHistoryLabel') }}</label>
          <input v-model.number="form.min_history_quota" type="number" min="0" step="0.01" class="input" />
        </div>
      </div>

      <div class="mt-5 grid gap-3 sm:grid-cols-2">
        <label class="flex items-center gap-3 rounded border border-gray-200 p-3 text-sm dark:border-dark-700">
          <input v-model="form.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
          <span>{{ t('admin.affiliates.levels.enabled') }}</span>
        </label>
        <label class="flex items-center gap-3 rounded border border-gray-200 p-3 text-sm dark:border-dark-700">
          <input v-model="form.is_default" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
          <span>{{ t('admin.affiliates.levels.defaultLevel') }}</span>
        </label>
      </div>

      <template #footer>
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="closeDialog">
            {{ t('common.cancel') }}
          </button>
          <button type="button" class="btn btn-primary" :disabled="saving || !canSubmit" @click="submitLevel">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="disableDialogOpen"
      :title="t('admin.affiliates.levels.disableTitle')"
      :message="disableTarget ? t('admin.affiliates.levels.disableMessage', { name: disableTarget.name }) : ''"
      :confirm-text="t('admin.affiliates.levels.disable')"
      danger
      @confirm="disableLevel"
      @cancel="disableDialogOpen = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { affiliatesAPI, type AffiliateAgentLevel, type UpsertAffiliateAgentLevelRequest } from '@/api/admin/affiliates'
import { extractI18nErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)
const levels = ref<AffiliateAgentLevel[]>([])
const dialogOpen = ref(false)
const editingLevel = ref<AffiliateAgentLevel | null>(null)
const disableDialogOpen = ref(false)
const disableTarget = ref<AffiliateAgentLevel | null>(null)

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.affiliates.levels.level'), sortable: true },
  { key: 'rebate_rate_percent', label: t('admin.affiliates.levels.rate'), sortable: true },
  { key: 'threshold', label: t('admin.affiliates.levels.threshold') },
  { key: 'sort_order', label: t('admin.affiliates.levels.sortOrder'), sortable: true },
  { key: 'enabled', label: t('common.status'), sortable: true },
  { key: 'actions', label: t('common.actions') },
])

const emptyForm = (): UpsertAffiliateAgentLevelRequest => ({
  code: '',
  name: '',
  rebate_rate_percent: 10,
  min_invited_count: 0,
  min_history_quota: 0,
  sort_order: nextSortOrder(),
  enabled: true,
  is_default: false,
})

const form = reactive<UpsertAffiliateAgentLevelRequest>(emptyForm())

const canSubmit = computed(() => {
  const code = form.code.trim()
  const name = form.name.trim()
  return (
    code.length >= 2 &&
    name.length > 0 &&
    Number.isFinite(form.rebate_rate_percent) &&
    form.rebate_rate_percent >= 0 &&
    form.rebate_rate_percent <= 100 &&
    Number.isFinite(form.min_invited_count) &&
    form.min_invited_count >= 0 &&
    Number.isFinite(form.min_history_quota) &&
    form.min_history_quota >= 0
  )
})

function nextSortOrder(): number {
  return levels.value.reduce((max, level) => Math.max(max, level.sort_order), 0) + 10
}

function assignForm(payload: UpsertAffiliateAgentLevelRequest) {
  Object.assign(form, payload)
}

async function loadLevels() {
  loading.value = true
  try {
    levels.value = await affiliatesAPI.listAgentLevels(true)
  } catch (error) {
    appStore.showError(extractI18nErrorMessage(error, t, 'admin.affiliates.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

function openCreateDialog() {
  editingLevel.value = null
  assignForm(emptyForm())
  dialogOpen.value = true
}

function openEditDialog(level: AffiliateAgentLevel) {
  editingLevel.value = level
  assignForm({
    code: level.code,
    name: level.name,
    rebate_rate_percent: level.rebate_rate_percent,
    min_invited_count: level.min_invited_count,
    min_history_quota: level.min_history_quota,
    sort_order: level.sort_order,
    enabled: level.enabled,
    is_default: level.is_default,
  })
  dialogOpen.value = true
}

function closeDialog() {
  dialogOpen.value = false
}

function buildPayload(): UpsertAffiliateAgentLevelRequest {
  return {
    code: form.code.trim().toUpperCase(),
    name: form.name.trim(),
    rebate_rate_percent: Number(form.rebate_rate_percent),
    min_invited_count: Math.max(0, Math.floor(Number(form.min_invited_count) || 0)),
    min_history_quota: Math.max(0, Number(form.min_history_quota) || 0),
    sort_order: Math.floor(Number(form.sort_order) || 0),
    enabled: Boolean(form.enabled),
    is_default: Boolean(form.is_default),
  }
}

async function submitLevel() {
  if (!canSubmit.value) return
  saving.value = true
  try {
    const payload = buildPayload()
    if (editingLevel.value) {
      await affiliatesAPI.updateAgentLevel(editingLevel.value.id, payload)
    } else {
      await affiliatesAPI.createAgentLevel(payload)
    }
    appStore.showSuccess(t('common.saved'))
    dialogOpen.value = false
    await loadLevels()
  } catch (error) {
    appStore.showError(extractI18nErrorMessage(error, t, 'admin.affiliates.errors', t('common.error')))
  } finally {
    saving.value = false
  }
}

function confirmDisable(level: AffiliateAgentLevel) {
  disableTarget.value = level
  disableDialogOpen.value = true
}

async function disableLevel() {
  const level = disableTarget.value
  if (!level) return
  try {
    await affiliatesAPI.deleteAgentLevel(level.id)
    appStore.showSuccess(t('common.saved'))
    disableDialogOpen.value = false
    disableTarget.value = null
    await loadLevels()
  } catch (error) {
    appStore.showError(extractI18nErrorMessage(error, t, 'admin.affiliates.errors', t('common.error')))
  }
}

function formatPercent(value: number): string {
  const rounded = Math.round(Number(value || 0) * 100) / 100
  return `${rounded}%`
}

function formatAmount(value: number): string {
  return Number(value || 0).toFixed(2)
}

onMounted(() => {
  void loadLevels()
})
</script>
