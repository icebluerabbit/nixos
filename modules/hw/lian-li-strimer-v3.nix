/*
  lian-li-strimer-v3 — Lian Li Strimer V3 ARGB extensions configuration and control via OpenRGB
  https://gitlab.com/CalcProgrammer1/OpenRGB
*/
{ den, ... }:
{
  den.aspects.hw._.lian-li-strimer-v3 = {
    aspect-name = "hw.lian-li-strimer-v3";
    includes = [ den.aspects.hw._.rgb ];
  };
}
