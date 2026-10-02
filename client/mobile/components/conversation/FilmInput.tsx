import { Controller, useFormContext } from "react-hook-form";
import InputField from "@/components/InputField";

export default function FilmInput() {
  const { control } = useFormContext();
  return (
    <Controller
      name="film"
      control={control}
      render={({ field: { onChange, value } }) => (
        <InputField
          variant="standard"
          label="Film(optional)"
          placeholder="Cure The Godfather Seven"
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
