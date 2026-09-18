<script setup>
import SiteManage from '../../components/SiteManage.vue'
defineProps({ id: String })
</script>
<template>
  <SiteManage :id="id" :admin="false" />
</template>
