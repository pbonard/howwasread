import { Controller, useFormContext } from "react-hook-form";
import InputField from "@/components/InputField";

export default function PlayInput() {
  const { control } = useFormContext();
  return (
    <Controller
      control={control}
      name="play"
      render={({ field: { onChange, value } }) => (
        <InputField
          variant="standard"
          label="Play(optional)"
          placeholder="Hamlet The Seagull The Glass Menagerie"
          inputMode="text"
          returnKeyType="done"
          submitBehavior="blurAndSubmit"
          value={value}
          onChangeText={onChange}
          maxLength={50}
        />
      )}
    />
  );
}
