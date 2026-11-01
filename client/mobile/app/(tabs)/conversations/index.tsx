import { Platform, Pressable, StyleSheet, View } from 'react-native';
import { router } from 'expo-router';
import {
  SafeAreaView,
  useSafeAreaInsets,
} from 'react-native-safe-area-context';
import { Feather } from '@expo/vector-icons';
import { colors } from '@/constants';
import OnlineConversationList from '@/components/conversation/OnlineConversationList';
import OfflineConversationMap from '@/components/conversation/OfflineConversationMap';
import { useRef, useState } from 'react';
import Tab from '@/components/Tab';
import PagerView from 'react-native-pager-view';
import { GlassView, isLiquidGlassAvailable } from 'expo-glass-effect';

// liquid glass only exists on ios 26+, elsewhere the button is the plain white circle
const hasLiquidGlass = isLiquidGlassAvailable();

export default function ConversationsScreen() {
  const [currentTab, setCurrentTab] = useState(0);
  const pagerRef = useRef<PagerView | null>(null);
  const insets = useSafeAreaInsets();
  const createButtonBottom = Platform.OS === 'ios' ? insets.bottom + 16 : 67;

  const handlePressCreate = () =>
    router.push(`/${currentTab === 0 ? 'offline' : 'online'}/create`, {
      dangerouslySingular: true,
    });

  const handlePressTab = (index: number) => {
    pagerRef.current?.setPage(index);
    setCurrentTab(index);
  };
  return (
    <SafeAreaView
      style={styles.container}
      edges={Platform.OS === 'ios' ? [] : ['bottom']}
    >
      <View style={styles.tabContainer}>
        <Tab isActive={currentTab === 0} onPress={() => handlePressTab(0)}>
          Offline
        </Tab>
        <Tab isActive={currentTab === 1} onPress={() => handlePressTab(1)}>
          Online
        </Tab>
      </View>
      <PagerView
        ref={pagerRef}
        initialPage={0}
        style={{ flex: 1 }}
        onPageSelected={(e) => setCurrentTab(e.nativeEvent.position)}
        scrollEnabled={false}
      >
        <OfflineConversationMap key="1" isActive={currentTab === 0} />
        <OnlineConversationList key="2" isActive={currentTab === 1} />
      </PagerView>
      {hasLiquidGlass ? (
        // light like the tab bar, the glass would turn dark over the dark map
        <GlassView
          style={[styles.createButtonGlass, { bottom: createButtonBottom }]}
          colorScheme="light"
          isInteractive
        >
          <Pressable
            style={styles.createButtonPressable}
            onPress={handlePressCreate}
          >
            <Feather name="plus" size={32} color="black" />
          </Pressable>
        </GlassView>
      ) : (
        <Pressable
          style={[styles.createButton, { bottom: createButtonBottom }]}
          onPress={handlePressCreate}
        >
          <Feather name="plus" size={32} color="black" />
        </Pressable>
      )}
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: colors.SAND_110,
  },
  createButton: {
    position: 'absolute',
    right: 16,
    backgroundColor: colors.WHITE,
    width: 64,
    height: 64,
    borderRadius: 32,
    justifyContent: 'center',
    alignItems: 'center',
    shadowColor: colors.BLACK,
    shadowOffset: { width: 0, height: 2 },
    shadowRadius: 3,
    shadowOpacity: 0.5,
    elevation: 2,
  },
  createButtonGlass: {
    position: 'absolute',
    right: 16,
    width: 64,
    height: 64,
    borderRadius: 32,
  },
  createButtonPressable: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
  },
  tabContainer: {
    flexDirection: 'row',
    marginTop: 8,
  },
});
