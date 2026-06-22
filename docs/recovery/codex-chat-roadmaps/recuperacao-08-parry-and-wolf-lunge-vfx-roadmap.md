# Recuperacao 08 - Parry Runtime And Wolf Lunge VFX

Thread: `019ec2b0-efc6-76d2-8905-b4a7469c3d65`

## Fonte Extraida Do Chat

- Problema inicial do thread:
  - creature atacava block, mas nunca tomava parry.
- Roadmap implicito:
  - parry deve ser resolvido pela pipeline autoritativa de dano/defesa;
  - block/parry precisam ser baseados em janela, direcao, contato e contrato;
  - creature skill que bate em parry valido deve receber evento/estado consequente.
- VFX:
  - Niagara `NS_Wolf_Lunge_GroundDust_WarmGrayBrown_NaturalV1` precisava trocar preto/cinza fosco para marrom quente natural.
  - Builder deve gerar paleta por layer, nao branco generico dependente de material.

## Status Atual A Validar

- Conferir se parry ainda existe na pipeline e se possui testes.
- Conferir se VFX builder atual conserva paleta quente.

