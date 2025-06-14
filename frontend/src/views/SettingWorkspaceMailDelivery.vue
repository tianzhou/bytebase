<template>
  <FormLayout>
    <template #body>
      <div class="w-full space-y-4">
        <div class="textinfolabel">
          {{ $t("settings.mail-delivery.description") }}
          <a
            class="normal-link inline-flex items-center"
            href="https://docs.bytebase.com/administration/mail-delivery?source=console"
            target="__BLANK"
          >
            {{ $t("common.learn-more") }}
            <heroicons-outline:external-link class="w-4 h-4 ml-1" />
          </a>
        </div>
        <div class="w-full flex flex-col gap-y-6">
          <!-- Host and Port -->
          <div class="w-full flex flex-row gap-4">
            <div class="min-w-max w-80">
              <div class="textlabel pl-1">
                {{ $t("settings.mail-delivery.field.smtp-server-host") }}
                <span class="text-red-600">*</span>
              </div>
              <NInput
                v-model:value="state.mailDeliverySetting.server"
                class="text-main w-full h-max mt-2"
                :placeholder="'smtp.gmail.com'"
              />
            </div>
            <div class="min-w-max w-48">
              <div class="textlabel pl-1">
                {{ $t("settings.mail-delivery.field.smtp-server-port") }}
                <span class="text-red-600">*</span>
              </div>
              <NInputNumber
                id="port"
                v-model:value="state.mailDeliverySetting.port"
                :show-button="false"
                :placeholder="'587'"
                :required="true"
                name="port"
                class="text-main w-full h-max mt-2 rounded-md border-control-border focus:ring-control focus:border-control disabled:bg-gray-50"
              />
            </div>
          </div>
          <div class="w-full flex flex-row gap-4">
            <div class="min-w-max w-80">
              <div class="textlabel pl-1">
                {{ $t("settings.mail-delivery.field.from") }}
                <span class="text-red-600">*</span>
              </div>
              <NInput
                v-model:value="state.mailDeliverySetting.from"
                class="text-main w-full h-max mt-2"
                :placeholder="'from@gmail.com'"
              />
            </div>
          </div>
          <!-- Authentication Related -->
          <div class="w-full gap-4">
            <div class="min-w-max w-80">
              <div class="textlabel pl-1">
                {{ $t("settings.mail-delivery.field.authentication-method") }}
              </div>
              <NSelect
                class="mt-2"
                :value="getSelectedAuthenticationTypeItem"
                :options="authenticationTypeOptions"
                :virtual-scroll="true"
                :fallback-option="false"
                @update:value="handleSelectAuthenticationType"
              />
            </div>
          </div>
          <!-- Not NONE Authentication-->
          <template
            v-if="
              state.mailDeliverySetting.authentication !==
              SMTPMailDeliverySettingValue_Authentication.AUTHENTICATION_NONE
            "
          >
            <div class="w-full flex flex-row gap-4">
              <div class="min-w-max w-80">
                <div class="flex flex-row">
                  <label class="textlabel pl-1">
                    {{ $t("settings.mail-delivery.field.smtp-username") }}
                    <span class="text-red-600">*</span>
                  </label>
                </div>
                <NInput
                  v-model:value="state.mailDeliverySetting.username"
                  class="text-main w-full h-max mt-2"
                  :placeholder="SYSTEM_BOT_EMAIL"
                />
              </div>
              <div class="min-w-max w-80">
                <div class="flex flex-row space-x-2">
                  <label class="textlabel pl-1">
                    {{ $t("settings.mail-delivery.field.smtp-password") }}
                    <span class="text-red-600">*</span>
                  </label>
                  <NCheckbox
                    :label="$t('common.empty')"
                    :checked="state.useEmptyPassword"
                    @update:checked="handleToggleUseEmptyPassword"
                  />
                </div>
                <BBTextField
                  v-model:value="state.mailDeliverySetting.password"
                  class="text-main w-full h-max mt-2"
                  :disabled="state.useEmptyPassword"
                  :required="isCreating"
                  :placeholder="'PASSWORD - INPUT_ONLY'"
                />
              </div>
            </div>
          </template>
          <!-- Encryption Related -->
          <div class="w-full gap-4">
            <div class="min-w-max w-80">
              <div class="textlabel pl-1">
                {{ $t("settings.mail-delivery.field.encryption") }}
              </div>
              <NSelect
                class="mt-2"
                :value="getSelectedEncryptionTypeItem"
                :options="encryptionTypeOptions"
                :virtual-scroll="true"
                :fallback-option="false"
                @update:value="handleSelectEncryptionType"
              />
            </div>
          </div>
          <!-- Test Send Email To Someone -->
          <div class="w-full gap-4 flex flex-row">
            <div class="min-w-max w-160">
              <div class="textlabel pl-1">
                {{ $t("settings.mail-delivery.field.send-test-email-to") }}
              </div>
              <div
                class="flex flex-row justify-start items-center mt-2 space-x-4"
              >
                <NInput
                  v-model:value="state.testMailTo"
                  class="text-main h-max w-80"
                  :placeholder="'someone@gmail.com'"
                />
                <NButton
                  type="primary"
                  :disabled="
                    state.testMailTo === '' || state.isCreateOrUpdateLoading
                  "
                  :loading="state.isSendLoading"
                  @click.prevent="testMailDeliverySetting"
                >
                  {{ $t("settings.mail-delivery.field.send") }}
                </NButton>
                <BBSpin v-if="state.isSendLoading" class="ml-2" />
              </div>
            </div>
          </div>
        </div>
      </div>
    </template>
    <template #footer>
      <div class="flex justify-end items-center">
        <NButton
          v-if="
            !isCreating &&
            !isEqual(state.mailDeliverySetting, state.originMailDeliverySetting)
          "
          @click="discardChanges"
        >
          {{ $t("common.discard-changes") }}
        </NButton>
        <NButton
          type="primary"
          :disabled="!allowMailDeliveryActionButton || state.isSendLoading"
          :loading="state.isCreateOrUpdateLoading"
          @click.prevent="updateMailDeliverySetting"
        >
          {{ mailDeliverySettingButtonText }}
        </NButton>
      </div>
    </template>
  </FormLayout>
</template>

<script lang="ts" setup>
import { cloneDeep, isEqual } from "lodash-es";
import type { SelectOption } from "naive-ui";
import { NButton, NCheckbox, NInput, NInputNumber, NSelect } from "naive-ui";
import type { ClientError } from "nice-grpc-web";
import { computed, onMounted, reactive } from "vue";
import { useI18n } from "vue-i18n";
import { BBSpin, BBTextField } from "@/bbkit";
import FormLayout from "@/components/v2/Form/FormLayout.vue";
import { pushNotification } from "@/store";
import { useWorkspaceMailDeliverySettingStore } from "@/store/modules/workspaceMailDeliverySetting";
import { SYSTEM_BOT_EMAIL } from "@/types";
import type { SMTPMailDeliverySettingValue } from "@/types/proto/v1/setting_service";
import {
  SMTPMailDeliverySettingValue_Authentication,
  SMTPMailDeliverySettingValue_Encryption,
} from "@/types/proto/v1/setting_service";

interface LocalState {
  originMailDeliverySetting?: SMTPMailDeliverySettingValue;
  mailDeliverySetting: SMTPMailDeliverySettingValue;
  testMailTo: string;
  isSendLoading: boolean;
  isCreateOrUpdateLoading: boolean;
  useEmptyPassword: boolean;
}

const props = defineProps<{
  allowEdit: boolean;
}>();

const { t } = useI18n();

const defaultMailDeliverySetting = function (): SMTPMailDeliverySettingValue {
  return {
    server: "",
    port: 587,
    username: "",
    password: undefined,
    from: "",
    authentication:
      SMTPMailDeliverySettingValue_Authentication.AUTHENTICATION_PLAIN,
    encryption: SMTPMailDeliverySettingValue_Encryption.ENCRYPTION_STARTTLS,
    to: "",
  };
};

const state = reactive<LocalState>({
  mailDeliverySetting: defaultMailDeliverySetting(),
  testMailTo: "",
  isSendLoading: false,
  isCreateOrUpdateLoading: false,
  useEmptyPassword: false,
});

const store = useWorkspaceMailDeliverySettingStore();

const isCreating = computed(
  () => state.originMailDeliverySetting === undefined
);

const discardChanges = () => {
  const setting = store.mailDeliverySetting;
  state.originMailDeliverySetting = cloneDeep(setting);
  if (state.originMailDeliverySetting) {
    state.mailDeliverySetting = cloneDeep(state.originMailDeliverySetting!);
  }
};

const mailDeliverySettingButtonText = computed(() => {
  return isCreating.value ? t("common.create") : t("common.update");
});

const allowMailDeliveryActionButton = computed(() => {
  if (!props.allowEdit) {
    return false;
  }
  if (
    !state.mailDeliverySetting.server ||
    !state.mailDeliverySetting.port ||
    !state.mailDeliverySetting.from
  ) {
    return false;
  }

  if (
    state.mailDeliverySetting.authentication !==
    SMTPMailDeliverySettingValue_Authentication.AUTHENTICATION_NONE
  ) {
    if (!state.mailDeliverySetting.username) {
      return false;
    }
    if (!state.useEmptyPassword && !state.mailDeliverySetting.password) {
      return false;
    }
  }

  return true;
});

onMounted(async () => {
  try {
    await store.fetchMailDeliverySetting();
    discardChanges();
  } catch {
    // nothing
  }
});

const updateMailDeliverySetting = async () => {
  state.isCreateOrUpdateLoading = true;
  const mailDelivery = cloneDeep(state.mailDeliverySetting);
  try {
    const value = cloneDeep(mailDelivery);
    await store.updateMailDeliverySetting(value);
  } catch (error) {
    state.isCreateOrUpdateLoading = false;
    pushNotification({
      module: "bytebase",
      style: "CRITICAL",
      title: `Request error occurred`,
      description: (error as ClientError).details,
    });
    return;
  }

  const currentValue = cloneDeep(store.mailDeliverySetting);
  state.originMailDeliverySetting = cloneDeep(currentValue);
  state.useEmptyPassword = false;
  if (state.originMailDeliverySetting) {
    state.mailDeliverySetting = cloneDeep(state.originMailDeliverySetting!);
  }
  state.isCreateOrUpdateLoading = false;
  pushNotification({
    module: "bytebase",
    style: "SUCCESS",
    title: t("settings.mail-delivery.updated-tip"),
  });
};

const testMailDeliverySetting = async () => {
  state.isSendLoading = true;
  const mailDelivery = cloneDeep(state.mailDeliverySetting);
  mailDelivery.to = state.testMailTo;
  try {
    await store.validateMailDeliverySetting(mailDelivery);
  } catch (error) {
    state.isSendLoading = false;
    pushNotification({
      module: "bytebase",
      style: "CRITICAL",
      title: `Request error occurred`,
      description: (error as ClientError).details,
    });
    return;
  }
  state.isSendLoading = false;
  pushNotification({
    module: "bytebase",
    style: "SUCCESS",
    title: t("settings.mail-delivery.tested-tip", { address: mailDelivery.to }),
  });
};

const handleSelectEncryptionType = (method: string) => {
  switch (method) {
    case "NONE":
      state.mailDeliverySetting.encryption =
        SMTPMailDeliverySettingValue_Encryption.ENCRYPTION_NONE;
      break;
    case "SSL/TLS":
      state.mailDeliverySetting.encryption =
        SMTPMailDeliverySettingValue_Encryption.ENCRYPTION_SSL_TLS;
      break;
    case "STARTTLS":
      state.mailDeliverySetting.encryption =
        SMTPMailDeliverySettingValue_Encryption.ENCRYPTION_STARTTLS;
      break;
    default:
      state.mailDeliverySetting.encryption =
        SMTPMailDeliverySettingValue_Encryption.ENCRYPTION_NONE;
      break;
  }
};

const encryptionTypeOptions = computed((): SelectOption[] => {
  return ["NONE", "SSL/TLS", "STARTTLS"].map((item) => ({
    value: item,
    label: item,
  }));
});

const authenticationTypeOptions = computed((): SelectOption[] => {
  return ["NONE", "PLAIN", "LOGIN", "CRAM-MD5"].map((item) => ({
    value: item,
    label: item,
  }));
});

const getSelectedEncryptionTypeItem = computed(() => {
  switch (state.mailDeliverySetting.encryption) {
    case SMTPMailDeliverySettingValue_Encryption.ENCRYPTION_NONE:
      return "NONE";
    case SMTPMailDeliverySettingValue_Encryption.ENCRYPTION_SSL_TLS:
      return "SSL/TLS";
    case SMTPMailDeliverySettingValue_Encryption.ENCRYPTION_STARTTLS:
      return "STARTTLS";
    default:
      return "NONE";
  }
});

const handleSelectAuthenticationType = (method: string) => {
  switch (method) {
    case "NONE":
      state.mailDeliverySetting.authentication =
        SMTPMailDeliverySettingValue_Authentication.AUTHENTICATION_NONE;
      break;
    case "PLAIN":
      state.mailDeliverySetting.authentication =
        SMTPMailDeliverySettingValue_Authentication.AUTHENTICATION_PLAIN;
      break;
    case "LOGIN":
      state.mailDeliverySetting.authentication =
        SMTPMailDeliverySettingValue_Authentication.AUTHENTICATION_LOGIN;
      break;
    case "CRAM-MD5":
      state.mailDeliverySetting.authentication =
        SMTPMailDeliverySettingValue_Authentication.AUTHENTICATION_CRAM_MD5;
      break;
    default:
      state.mailDeliverySetting.authentication =
        SMTPMailDeliverySettingValue_Authentication.AUTHENTICATION_PLAIN;
      break;
  }
};
const getSelectedAuthenticationTypeItem = computed(() => {
  switch (state.mailDeliverySetting.authentication) {
    case SMTPMailDeliverySettingValue_Authentication.AUTHENTICATION_NONE:
      return "NONE";
    case SMTPMailDeliverySettingValue_Authentication.AUTHENTICATION_PLAIN:
      return "PLAIN";
    case SMTPMailDeliverySettingValue_Authentication.AUTHENTICATION_LOGIN:
      return "LOGIN";
    case SMTPMailDeliverySettingValue_Authentication.AUTHENTICATION_CRAM_MD5:
      return "CRAM-MD5";
    default:
      return "PLAIN";
  }
});

const handleToggleUseEmptyPassword = (on: boolean) => {
  state.useEmptyPassword = on;
  if (on) {
    state.mailDeliverySetting.password = "";
  } else {
    state.mailDeliverySetting.password = undefined;
  }
};
</script>
