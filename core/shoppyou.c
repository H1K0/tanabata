#include <malloc.h>
#include <string.h>
#include <time.h>

#include "../include/core.h"

const Kazari HOLE_KAZARI = {HOLE_ID};

// Shoppyou file signature: 七夕飾表
static const uint16_t SHOPPYOU_SIG[4] = {L'七', L'夕', L'飾', L'表'};

int shoppyou_init(Shoppyou *shoppyou) {
    shoppyou->created_ts = time(NULL);
    shoppyou->modified_ts = shoppyou->created_ts;
    shoppyou->size = 0;
    shoppyou->database = NULL;
    shoppyou->hole_cnt = 0;
    shoppyou->holes = NULL;
    shoppyou->file = NULL;
    return 0;
}

int shoppyou_free(Shoppyou *shoppyou) {
    free(shoppyou->database);
    free(shoppyou->holes);
    if (shoppyou->file != NULL) {
        fclose(shoppyou->file);
    }
    return 0;
}

int shoppyou_load(Shoppyou *shoppyou) {
    shoppyou->file = freopen(NULL, "rb", shoppyou->file);
    if (shoppyou->file == NULL) {
        return 1;
    }
    uint16_t signature[4];
    if (fread(signature, 2, 4, shoppyou->file) < 4 ||
        memcmp(signature, SHOPPYOU_SIG, 8) != 0 ||
        fread(&shoppyou->created_ts, 8, 1, shoppyou->file) == 0 ||
        fread(&shoppyou->modified_ts, 8, 1, shoppyou->file) == 0 ||
        fread(&shoppyou->size, 8, 1, shoppyou->file) == 0) {
        return 1;
    }
    shoppyou->hole_cnt = 0;
    free(shoppyou->holes);
    shoppyou->database = malloc(shoppyou->size * sizeof(Kazari));
    for (uint64_t i = 0; i < shoppyou->size; i++) {
        if (fread(&shoppyou->database[i].created_ts, 8, 1, shoppyou->file) == 0 ||
            fread(&shoppyou->database[i].sasa_id, 8, 1, shoppyou->file) == 0 ||
            fread(&shoppyou->database[i].tanzaku_id, 8, 1, shoppyou->file) == 0) {
            return 1;
        }
    }
    return fflush(shoppyou->file);
}

int shoppyou_save(Shoppyou *shoppyou) {
    shoppyou->file = freopen(NULL, "wb", shoppyou->file);
    if (shoppyou->file == NULL ||
        fwrite(SHOPPYOU_SIG, 2, 4, shoppyou->file) < 4 ||
        fwrite(&shoppyou->created_ts, 8, 1, shoppyou->file) == 0 ||
        fwrite(&shoppyou->modified_ts, 8, 1, shoppyou->file) == 0) {
        return 1;
    }
    uint64_t size = shoppyou->size - shoppyou->hole_cnt;
    if (fwrite(&size, 8, 1, shoppyou->file) == 0 ||
        fflush(shoppyou->file) != 0) {
        return 1;
    }
    for (uint64_t i = 0; i < shoppyou->size; i++) {
        if (shoppyou->database[i].sasa_id != HOLE_ID && shoppyou->database[i].tanzaku_id != HOLE_ID) {
            if (fwrite(&shoppyou->database[i].created_ts, 8, 1, shoppyou->file) == 0 ||
                fwrite(&shoppyou->database[i].sasa_id, 8, 1, shoppyou->file) == 0 ||
                fwrite(&shoppyou->database[i].tanzaku_id, 8, 1, shoppyou->file) == 0) {
                return 1;
            }
        }
    }
    return fflush(shoppyou->file);
}

int shoppyou_open(Shoppyou *shoppyou, const char *path) {
    shoppyou->file = fopen(path, "rb");
    if (shoppyou->file == NULL) {
        return 1;
    }
    shoppyou->holes = NULL;
    return shoppyou_load(shoppyou);
}

int shoppyou_dump(Shoppyou *shoppyou, const char *path) {
    shoppyou->file = fopen(path, "wb");
    if (shoppyou->file == NULL) {
        return 1;
    }
    return shoppyou_save(shoppyou);
}

int kazari_add(Shoppyou *shoppyou, uint64_t sasa_id, uint64_t tanzaku_id) {
    if (sasa_id == HOLE_ID || tanzaku_id == HOLE_ID) {
        return 1;
    }
    if (shoppyou->size == -1 && shoppyou->hole_cnt == 0) {
        return 1;
    }
    Kazari newbie;
    newbie.created_ts = time(NULL);
    newbie.sasa_id = sasa_id;
    newbie.tanzaku_id = tanzaku_id;
    if (shoppyou->hole_cnt > 0) {
        shoppyou->hole_cnt--;
        **(shoppyou->holes + shoppyou->hole_cnt) = newbie;
        shoppyou->holes = realloc(shoppyou->holes, shoppyou->hole_cnt * sizeof(Kazari *));
    } else {
        shoppyou->size++;
        shoppyou->database = realloc(shoppyou->database, shoppyou->size * sizeof(Kazari));
        shoppyou->database[shoppyou->size - 1] = newbie;
    }
    shoppyou->modified_ts = newbie.created_ts;
    return 0;
}

int kazari_rem(Shoppyou *shoppyou, uint64_t sasa_id, uint64_t tanzaku_id) {
    if (sasa_id == HOLE_ID || tanzaku_id == HOLE_ID) {
        return 1;
    }
    for (uint64_t i = 0; i < shoppyou->size; i++) {
        if (shoppyou->database[i].sasa_id == sasa_id && shoppyou->database[i].tanzaku_id == tanzaku_id) {
            shoppyou->database[i].sasa_id = HOLE_ID;
            shoppyou->hole_cnt++;
            shoppyou->holes = realloc(shoppyou->holes, shoppyou->hole_cnt * sizeof(Kazari *));
            shoppyou->holes[shoppyou->hole_cnt - 1] = shoppyou->database + i;
            shoppyou->modified_ts = time(NULL);
            return 0;
        }
    }
    return 0;
}

int kazari_rem_by_sasa(Shoppyou *shoppyou, uint64_t sasa_id) {
    if (sasa_id == HOLE_ID) {
        return 1;
    }
    _Bool changed = 0;
    for (uint64_t i = 0; i < shoppyou->size; i++) {
        if (shoppyou->database[i].sasa_id == sasa_id) {
            shoppyou->database[i].sasa_id = HOLE_ID;
            shoppyou->hole_cnt++;
            shoppyou->holes = realloc(shoppyou->holes, shoppyou->hole_cnt * sizeof(Kazari *));
            shoppyou->holes[shoppyou->hole_cnt - 1] = shoppyou->database + i;
            changed = 1;
        }
    }
    if (changed) {
        shoppyou->modified_ts = time(NULL);
    }
    return 0;
}

int kazari_rem_by_tanzaku(Shoppyou *shoppyou, uint64_t tanzaku_id) {
    if (tanzaku_id == HOLE_ID) {
        return 1;
    }
    _Bool changed = 0;
    for (uint64_t i = 0; i < shoppyou->size; i++) {
        if (shoppyou->database[i].tanzaku_id == tanzaku_id) {
            shoppyou->database[i].tanzaku_id = HOLE_ID;
            shoppyou->hole_cnt++;
            shoppyou->holes = realloc(shoppyou->holes, shoppyou->hole_cnt * sizeof(Kazari *));
            shoppyou->holes[shoppyou->hole_cnt - 1] = shoppyou->database + i;
            changed = 1;
        }
    }
    if (changed) {
        shoppyou->modified_ts = time(NULL);
    }
    return 0;
}
