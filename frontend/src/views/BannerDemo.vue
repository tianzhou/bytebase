<template>
  <div v-if="show" class="bg-accent">
    <div class="mx-auto py-1 px-3">
      <div class="flex items-center justify-between flex-wrap">
        <div class="w-0 flex-1 flex items-center">
          <p class="flex font-medium text-white items-center truncate">
            📆
            <a
              href="https://cal.com/bytebase/product-walkthrough"
              target="_blank"
              class="flex underline ml-1"
            >
              {{ $t("banner.request-demo") }}
            </a>
          </p>
        </div>
        <div
          class="flex flex-row space-x-4 order-3 my-2 flex-shrink-0 w-full sm:order-2 sm:py-1 sm:my-0 sm:w-auto"
        >
          <a
            href="https://docs.bytebase.com/get-started/deploy-with-docker/?source=demo"
            target="_blank"
            class="flex items-center justify-center pl-4 pr-2 py-1 border border-transparent rounded-sm text-base font-medium text-accent bg-white hover:bg-indigo-50"
          >
            {{ $t("banner.deploy") }}
            <heroicons-outline:arrow-down-tray class="ml-1 w-6 h-6" />
          </a>
          <a
            href="https://hub.bytebase.com/workspace?source=demo"
            target="_blank"
            class="flex items-center justify-center pl-4 pr-2 py-1 border border-transparent rounded-sm text-base font-medium text-accent bg-white hover:bg-indigo-50"
          >
            {{ $t("banner.cloud") }}
            <heroicons-outline:cloud class="ml-1 w-6 h-6" />
          </a>
        </div>

        <div class="order-2 flex-shrink-0 sm:order-3 sm:ml-3 -mr-1">
          <NButton quaternary size="small" @click.prevent="show = false">
            <span class="sr-only">{{ $t("common.dismiss") }}</span>
            <template #icon>
              <!-- Heroicon name: outline/x -->
              <heroicons-outline:x class="h-6 w-6 text-white" />
            </template>
          </NButton>
        </div>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { NButton } from "naive-ui";
import { ref } from "vue";

const show = ref(true);
</script>
