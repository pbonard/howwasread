import { Controller, useFormContext } from "react-hook-form";
import InputField from "@/components/InputField";

export default function PoemInput() {
  const { control } = useFormContext();
  return (
    <Controller
      name="poem"
      control={control}
      render={({ field: { onChange, value } }) => (
        <InputField
          variant="standard"
          label="Poem(optional)"
          placeholder="The Waste Land Sonnet 18 If -"
          inputMode="text"
          returnKeyType="next"
          submitBehavior="submit"
          value={value}
          onChangeText={onChange}
          maxLength={50}
        />
      )}
    />
  );
}
