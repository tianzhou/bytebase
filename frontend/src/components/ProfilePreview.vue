<template>
  <component
    :is="link ? 'router-link' : 'div'"
    class="px-4 py-3 menu-item w-48"
    :to="{ name: SETTING_ROUTE_PROFILE }"
    role="menuitem"
  >
    <p class="text-sm flex justify-between space-x-2">
      <span class="text-main font-medium truncate text-ellipsis">
        {{ currentUserV1.title }}
      </span>
    </p>
    <p class="text-sm text-control truncate text-ellipsis">
      {{ currentUserV1.email }}
    </p>
  </component>
</template>

<script setup lang="ts">
import { SETTING_ROUTE_PROFILE } from "@/router/dashboard/workspaceSetting";
import { useCurrentUserV1 } from "@/store";

withDefaults(
  defineProps<{
    link?: boolean;
  }>(),
  {
    link: true,
  }
);

const currentUserV1 = useCurrentUserV1();
</script>
