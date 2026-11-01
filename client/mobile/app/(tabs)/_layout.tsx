import { NativeTabs } from 'expo-router/unstable-native-tabs';
import React from 'react';

// the system tab bar: liquid glass on ios 26, material on android.
// icons are sf symbols (ios) and material symbols (android), vector icons can't be drawn natively
export default function TabLayout() {
  return (
    <NativeTabs
      tintColor="blue"
      // ios 26 ignores the background color (glass), it only paints android and ios 18 and lower
      backgroundColor="white"
      minimizeBehavior="onScrollDown"
      // the app is drawn light, a phone in dark mode would turn the glass dark gray
      unstable_nativeProps={{ colorScheme: 'light' }}
    >
      <NativeTabs.Trigger name="conversations">
        <NativeTabs.Trigger.Label>Conversations</NativeTabs.Trigger.Label>
        <NativeTabs.Trigger.Icon sf="arrow.left.arrow.right" md="swap_horiz" />
      </NativeTabs.Trigger>
      <NativeTabs.Trigger name="chats">
        <NativeTabs.Trigger.Label>Chats</NativeTabs.Trigger.Label>
        <NativeTabs.Trigger.Icon
          sf={{
            default: 'bubble.left.and.bubble.right',
            selected: 'bubble.left.and.bubble.right.fill',
          }}
          md={{ default: 'chat_bubble_outline', selected: 'chat_bubble' }}
        />
      </NativeTabs.Trigger>
      <NativeTabs.Trigger name="account">
        <NativeTabs.Trigger.Label>Account</NativeTabs.Trigger.Label>
        <NativeTabs.Trigger.Icon
          sf={{ default: 'person', selected: 'person.fill' }}
          md={{ default: 'person_outline', selected: 'person' }}
        />
      </NativeTabs.Trigger>
    </NativeTabs>
  );
}
