#include <stdint.h>
#include <malloc.h>
#include <string.h>
#include <time.h>

#include "core_func.h"

const Kazari HOLE_KAZARI = {HOLE_ID, HOLE_ID, 0};

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
    shoppyou->created_ts = 0;
    shoppyou->modified_ts = 0;
    shoppyou->size = 0;
    shoppyou->hole_cnt = 0;
    free(shoppyou->database);
    shoppyou->database = NULL;
    free(shoppyou->holes);
    shoppyou->holes = NULL;
    if (shoppyou->file != NULL) {
        fclose(shoppyou->file);
        shoppyou->file = NULL;
    }
    return 0;
}

int shoppyou_load(Shoppyou *shoppyou) {
    if (shoppyou->file == NULL ||
        (shoppyou->file = freopen(NULL, "rb", shoppyou->file)) == NULL) {
        return 1;
    }
    Shoppyou temp;
    shoppyou_init(&temp);
    temp.file = shoppyou->file;
    uint16_t signature[4];
    if (fread(signature, 2, 4, temp.file) != 4 ||
        memcmp(signature, SHOPPYOU_SIG, 8) != 0 ||
        fread(&temp.created_ts, 8, 1, temp.file) != 1 ||
        fread(&temp.modified_ts, 8, 1, temp.file) != 1 ||
        fread(&temp.size, 8, 1, temp.file) != 1) {
        return 1;
    }
    temp.database = calloc(temp.size, sizeof(Kazari));
    Kazari *current_kazari = temp.database;
    for (uint64_t i = 0; i < temp.size; i++) {
        if (fread(&current_kazari->created_ts, 8, 1, temp.file) != 1 ||
            fread(&current_kazari->sasa_id, 8, 1, temp.file) != 1 ||
            fread(&current_kazari->tanzaku_id, 8, 1, temp.file) != 1) {
            temp.file = NULL;
            shoppyou_free(&temp);
            return 1;
        }
        current_kazari++;
    }
    if (fflush(temp.file) == 0) {
        shoppyou->file = NULL;
        shoppyou_free(shoppyou);
        *shoppyou = temp;
        return 0;
    }
    temp.file = NULL;
    shoppyou_free(&temp);
    return 1;
}

int shoppyou_save(Shoppyou *shoppyou) {
    if (shoppyou->file == NULL ||
        (shoppyou->file = freopen(NULL, "wb", shoppyou->file)) == NULL ||
        fwrite(SHOPPYOU_SIG, 2, 4, shoppyou->file) != 4 ||
        fwrite(&shoppyou->created_ts, 8, 1, shoppyou->file) != 1 ||
        fwrite(&shoppyou->modified_ts, 8, 1, shoppyou->file) != 1) {
        return 1;
    }
    uint64_t size = shoppyou->size - shoppyou->hole_cnt;
    if (fwrite(&size, 8, 1, shoppyou->file) != 1 ||
        fflush(shoppyou->file) != 0) {
        return 1;
    }
    Kazari *current_kazari = shoppyou->database;
    for (uint64_t i = 0; i < shoppyou->size; i++) {
        if (shoppyou->database[i].sasa_id != HOLE_ID && shoppyou->database[i].tanzaku_id != HOLE_ID) {
            if (fwrite(&current_kazari->created_ts, 8, 1, shoppyou->file) != 1 ||
                fwrite(&current_kazari->sasa_id, 8, 1, shoppyou->file) != 1 ||
                fwrite(&current_kazari->tanzaku_id, 8, 1, shoppyou->file) != 1) {
                return 1;
            }
        }
        current_kazari++;
    }
    return fflush(shoppyou->file);
}

int shoppyou_open(Shoppyou *shoppyou, const char *path) {
    if (path == NULL) {
        return 1;
    }
    if (shoppyou->file == NULL && (shoppyou->file = fopen(path, "rb")) == NULL ||
        shoppyou->file != NULL && (shoppyou->file = freopen(path, "rb", shoppyou->file)) == NULL) {
        return 1;
    }
    return shoppyou_load(shoppyou);
}

int shoppyou_dump(Shoppyou *shoppyou, const char *path) {
    if (path == NULL) {
        return 1;
    }
    if (shoppyou->file == NULL && (shoppyou->file = fopen(path, "wb")) == NULL ||
        shoppyou->file != NULL && (shoppyou->file = freopen(path, "wb", shoppyou->file)) == NULL) {
        return 1;
    }
    return shoppyou_save(shoppyou);
}

int kazari_add(Shoppyou *shoppyou, uint64_t sasa_id, uint64_t tanzaku_id) {
    if (sasa_id == HOLE_ID || tanzaku_id == HOLE_ID || shoppyou->size == -1 && shoppyou->hole_cnt == 0) {
        return 1;
    }
    Kazari newbie;
    newbie.created_ts = time(NULL);
    newbie.sasa_id = sasa_id;
    newbie.tanzaku_id = tanzaku_id;
    if (shoppyou->hole_cnt > 0) {
        shoppyou->hole_cnt--;
        **(shoppyou->holes + shoppyou->hole_cnt) = newbie;
        shoppyou->holes = reallocarray(shoppyou->holes, shoppyou->hole_cnt, sizeof(Kazari *));
    } else {
        shoppyou->size++;
        shoppyou->database = reallocarray(shoppyou->database, shoppyou->size, sizeof(Kazari));
        shoppyou->database[shoppyou->size - 1] = newbie;
    }
    shoppyou->modified_ts = newbie.created_ts;
    return 0;
}

int kazari_rem(Shoppyou *shoppyou, uint64_t sasa_id, uint64_t tanzaku_id) {
    if (sasa_id == HOLE_ID || tanzaku_id == HOLE_ID) {
        return 1;
    }
    Kazari *current_kazari = shoppyou->database;
    for (uint64_t i = 0; i < shoppyou->size; i++) {
        if (current_kazari->sasa_id == sasa_id && current_kazari->tanzaku_id == tanzaku_id) {
            current_kazari->sasa_id = HOLE_ID;
            current_kazari->tanzaku_id = HOLE_ID;
            shoppyou->hole_cnt++;
            shoppyou->holes = reallocarray(shoppyou->holes, shoppyou->hole_cnt, sizeof(Kazari *));
            shoppyou->holes[shoppyou->hole_cnt - 1] = current_kazari;
            shoppyou->modified_ts = time(NULL);
            return 0;
        }
        current_kazari++;
    }
    return 1;
}

int kazari_rem_by_sasa(Shoppyou *shoppyou, uint64_t sasa_id) {
    if (sasa_id == HOLE_ID) {
        return 1;
    }
    Kazari *current_kazari = shoppyou->database;
    _Bool changed = 0;
    for (uint64_t i = 0; i < shoppyou->size; i++) {
        if (current_kazari->sasa_id == sasa_id) {
            current_kazari->sasa_id = HOLE_ID;
            current_kazari->tanzaku_id = HOLE_ID;
            shoppyou->hole_cnt++;
            shoppyou->holes = reallocarray(shoppyou->holes, shoppyou->hole_cnt, sizeof(Kazari *));
            shoppyou->holes[shoppyou->hole_cnt - 1] = current_kazari;
            changed = 1;
        }
        current_kazari++;
    }
    if (changed) {
        shoppyou->modified_ts = time(NULL);
        return 0;
    }
    return 1;
}

int kazari_rem_by_tanzaku(Shoppyou *shoppyou, uint64_t tanzaku_id) {
    if (tanzaku_id == HOLE_ID) {
        return 1;
    }
    Kazari *current_kazari = shoppyou->database;
    _Bool changed = 0;
    for (uint64_t i = 0; i < shoppyou->size; i++) {
        if (current_kazari->tanzaku_id == tanzaku_id) {
            current_kazari->sasa_id = HOLE_ID;
            current_kazari->tanzaku_id = HOLE_ID;
            shoppyou->hole_cnt++;
            shoppyou->holes = reallocarray(shoppyou->holes, shoppyou->hole_cnt, sizeof(Kazari *));
            shoppyou->holes[shoppyou->hole_cnt - 1] = current_kazari;
            changed = 1;
        }
        current_kazari++;
    }
    if (changed) {
        shoppyou->modified_ts = time(NULL);
        return 0;
    }
    return 1;
}
