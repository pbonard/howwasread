import { Controller, useFormContext } from "react-hook-form";
import InputField from "@/components/InputField";

export default function WrittenBy() {
  const { control } = useFormContext();
  return (
    <Controller
      name="writtenBy"
      control={control}
      rules={{
        validate: (data: string) => {
          if (data.trim().length <= 0) {
            return "Written by is required field";
          }
        },
      }}
      render={({ field: { onChange, value }, fieldState: { error } }) => (
        <InputField
          variant="standard"
          label="written by(required)"
          placeholder="Shakespeare Soseki Pound"
          inputMode="text"
          returnKeyType="done"
          submitBehavior="blurAndSubmit"
          value={value}
          onChangeText={onChange}
          maxLength={50}
          error={error?.message}
        />
      )}
    />
  );
}
