import { Tabs } from 'expo-router';
import { NativeTabs } from 'expo-router/unstable-native-tabs';
import React from 'react';
import { Platform } from 'react-native';
import { FontAwesome, Ionicons, MaterialIcons } from '@expo/vector-icons';

export default function TabLayout() {
  return Platform.OS === 'ios' ? <IosTabs /> : <AndroidTabs />;
}

function IosTabs() {
  return (
    <NativeTabs
      tintColor="black"
      backgroundColor="white"
      minimizeBehavior="onScrollDown"
      unstable_nativeProps={{ colorScheme: 'light' }}
    >
      <NativeTabs.Trigger name="conversations">
        <NativeTabs.Trigger.Label>Conversations</NativeTabs.Trigger.Label>
        <NativeTabs.Trigger.Icon sf="arrow.left.arrow.right" />
      </NativeTabs.Trigger>
      <NativeTabs.Trigger name="chats">
        <NativeTabs.Trigger.Label>Chats</NativeTabs.Trigger.Label>
        <NativeTabs.Trigger.Icon
          sf={{
            default: 'bubble.left.and.bubble.right',
            selected: 'bubble.left.and.bubble.right.fill',
          }}
        />
      </NativeTabs.Trigger>
      <NativeTabs.Trigger name="account">
        <NativeTabs.Trigger.Label>Account</NativeTabs.Trigger.Label>
        <NativeTabs.Trigger.Icon
          sf={{ default: 'person', selected: 'person.fill' }}
        />
      </NativeTabs.Trigger>
    </NativeTabs>
  );
}

function AndroidTabs() {
  return (
    <Tabs
      screenOptions={{
        tabBarActiveTintColor: 'black',
        headerShown: false,
      }}
    >
      <Tabs.Screen
        name="conversations"
        options={{
          title: 'Conversations',
          tabBarIcon: ({ color }) => (
            <FontAwesome name="exchange" size={24} color={color} />
          ),
        }}
      />
      <Tabs.Screen
        name="chats"
        options={{
          title: 'Chats',
          tabBarIcon: ({ color, focused }) => (
            <Ionicons
              name={focused ? 'chatbubbles-sharp' : 'chatbubbles-outline'}
              size={24}
              color={color}
            />
          ),
        }}
      />
      <Tabs.Screen
        name="account"
        options={{
          title: 'Account',
          tabBarIcon: ({ color, focused }) => (
            <MaterialIcons
              name={focused ? 'person' : 'person-outline'}
              size={30}
              color={color}
            />
          ),
        }}
      />
    </Tabs>
  );
}
