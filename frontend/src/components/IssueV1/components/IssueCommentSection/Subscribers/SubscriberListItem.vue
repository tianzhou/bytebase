<template>
  <div class="flex items-center gap-x-2 py-1">
    <UserAvatar :user="user" size="NORMAL" />
    <div class="flex flex-col w-[10rem]">
      <span class="truncate">{{ user.title }}</span>
      <span class="truncate text-control-light text-xs">{{ user.email }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import UserAvatar from "@/components/User/UserAvatar.vue";
import { type User } from "@/types/proto/v1/user_service";

defineProps<{
  user: User;
}>();
</script>
