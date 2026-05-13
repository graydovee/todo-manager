import { createContext, useContext } from 'react';

interface LangContextType {
  lang: string;
  setLang: (lang: string) => void;
}

const LangContext = createContext<LangContextType>({
  lang: 'en',
  setLang: () => {},
});

export const LangProvider = LangContext.Provider;
export const useLang = () => useContext(LangContext);
