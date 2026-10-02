import { Controller, useFormContext } from "react-hook-form";
import InputField from "@/components/InputField";

export default function DescriptionInput() {
  const { control } = useFormContext();
  return (
    <Controller
      name="description"
      control={control}
      render={({ field: { onChange, value } }) => (
        <InputField
          variant="standard"
          label="Description(optional)"
          placeholder={
            "Why does Hamlet hesitate?\nWho is to blame for Ophelia's fate?"
          }
          inputMode="text"
          returnKeyType="default"
          submitBehavior="newline"
          value={value}
          onChangeText={onChange}
          multiline
          maxLength={500}
        />
      )}
    />
  );
}
