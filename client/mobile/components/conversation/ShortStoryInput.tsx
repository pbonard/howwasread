import { Controller, useFormContext } from "react-hook-form";
import InputField from "@/components/InputField";

export default function ShortStoryInput() {
  const { control } = useFormContext();

  return (
    <Controller
      name="shortStory"
      control={control}
      render={({ field: { onChange, value } }) => (
        <InputField
          variant="standard"
          label="ShortStory(optional)"
          placeholder={"Cathedral Barn Burning Araby"}
          inputMode="text"
          value={value}
          onChangeText={onChange}
          maxLength={50}
        />
      )}
    />
  );
}
