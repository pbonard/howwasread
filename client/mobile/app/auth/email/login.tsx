import { StyleSheet, View } from 'react-native';
import { FormProvider, useForm } from 'react-hook-form';
import FixedBottomCTA from '@/components/FixedBottomCTA';
import EmailInput from '@/components/auth/EmailInput';
import PasswordInput from '@/components/auth/PasswordInput';
import { useLoginWithEmail } from '@/hooks/useAuth';
import { colors } from '@/constants';
import { Link, router } from 'expo-router';
import { getMyProfile } from '@/api/profile';
import { setKVStore } from '@/db/storage';
import CustomButton from '@/components/CustomButton';

interface FormValue {
  email: string;
  password: string;
}

export default function LoginScreen() {
  const loginWithEmailMutation = useLoginWithEmail();

  const emailLoginForm = useForm<FormValue>({
    defaultValues: {
      email: '',
      password: '',
    },
  });

  const onSubmit = (formValues: FormValue) => {
    const { email, password } = formValues;

    loginWithEmailMutation.mutate(
      {
        email,
        password,
      },
      {
        onSuccess: async (data) => {
          if (data.verificationId) {
            router.replace('/auth/otp/email');
            return;
          }
          if (data.sessionId) {
            router.replace('/auth/phone-number');
            return;
          }
          if (data.accessToken) {
            const my = await getMyProfile();
            setKVStore('myId', my.id);
            if (!my.name) {
              router.replace('/profile/name');
              return;
            }
            setKVStore('myName', my.name);
            router.replace('/conversations');
          }
        },
      },
    );
  };
  return (
    <FormProvider {...emailLoginForm}>
      <View style={styles.container}>
        <View style={styles.content}>
          <EmailInput />
          <PasswordInput />
          <View style={{ marginTop: 50, gap: 40, alignItems: 'center' }}>
            <CustomButton
              label="login"
              onPress={emailLoginForm.handleSubmit(onSubmit)}
              disabled={loginWithEmailMutation.isPending}
            />
            <Link
              href="/auth/email/forget-password"
              dangerouslySingular={true}
              style={{
                color: colors.GRAY_500,
                textDecorationLine: 'underline',
              }}
            >
              Forgot your password?
            </Link>
          </View>
        </View>
      </View>
    </FormProvider>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: colors.SAND_110,
    borderTopWidth: StyleSheet.hairlineWidth,
    borderColor: colors.GRAY_700,
  },
  content: {
    flex: 1,
    margin: 16,
    gap: 16,
    paddingHorizontal: 20,
    backgroundColor: colors.SAND_110,
  },
});
