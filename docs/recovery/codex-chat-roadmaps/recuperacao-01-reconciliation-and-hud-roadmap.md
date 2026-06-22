# Recuperacao 01 - Reconciliation, Combat Mode HUD, Figma HUD

Thread: `019ee718-0dbd-7791-b75e-32015f3ca5d8`

## Fonte Extraida Do Chat

- Reconciliation nao deve ser um contrato totalmente separado por skill.
- Arquitetura correta:
  - global reconciliation engine;
  - reconciliation profile por familia/categoria de movimento;
  - skill movement contract referencia o profile;
  - overrides pequenos por contrato so quando fisicamente justificavel.
- Categorias citadas:
  - `locomotion_default`
  - `evasive_burst`
  - `airborne_commit`
  - `grounded_contact_rush`
  - `short_grounded_impulse`
  - `micro_lunge_commit`
- HUD source of truth no Figma:
  - vida esquerda;
  - stamina direita;
  - simbolo de modo no centro;
  - slots circulares;
  - consumiveis verticais;
  - windup azul solto;
  - assets exportaveis para Unreal UMG.

## Invariantes

- Server authoritative movement e `movement_action_contract` sao fonte de verdade.
- Unreal so apresenta, prediz e reconcilia conforme profile confirmado.
- Dodge, leap e turn nao podem ser alterados para consertar uma skill especifica.
- Combat mode visual deve seguir ACK/snapshot autoritativo, nunca fallback de tecla.

## Status Atual A Validar

- Confirmar se DB possui reconciliation profiles por categoria suficiente.
- Confirmar se cada `movement_action_contract` aponta para reconciliation profile apropriado.
- Confirmar se HUD atual nao usa fallback visual de skill rejeitada para trocar modo.
- Confirmar se Figma/HUD final foi refletido no Unreal ou permanece apenas design source.

