#include <malloc.h>
#include <string.h>
#include <time.h>

#include "../../include/core.h"

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
    if (shoppyou->file == NULL) {
        fprintf(stderr, "Failed to load shoppyou: file not specified\n");
        return 1;
    }
    uint16_t signature[4];
    rewind(shoppyou->file);
    fread(signature, 2, 4, shoppyou->file);
    if (memcmp(signature, SHOPPYOU_SIG, 8) != 0) {
        fprintf(stderr, "Failed to load shoppyou: invalid signature\n");
        return 1;
    }
    fread(&shoppyou->created_ts, 8, 1, shoppyou->file);
    fread(&shoppyou->modified_ts, 8, 1, shoppyou->file);
    fread(&shoppyou->size, 8, 1, shoppyou->file);
    shoppyou->hole_cnt = 0;
    free(shoppyou->holes);
    shoppyou->database = malloc(shoppyou->size * sizeof(Kazari));
    for (uint64_t i = 0; i < shoppyou->size; i++) {
        fread(&shoppyou->database[i].created_ts, 8, 1, shoppyou->file);
        fread(&shoppyou->database[i].sasa_id, 8, 1, shoppyou->file);
        fread(&shoppyou->database[i].tanzaku_id, 8, 1, shoppyou->file);
    }
    return 0;
}

int shoppyou_save(Shoppyou *shoppyou) {
    if (shoppyou->file == NULL) {
        fprintf(stderr, "Failed to save shoppyou: file not specified\n");
        return 1;
    }
    rewind(shoppyou->file);
    fwrite(SHOPPYOU_SIG, 2, 4, shoppyou->file);
    fwrite(&shoppyou->created_ts, 8, 1, shoppyou->file);
    fwrite(&shoppyou->modified_ts, 8, 1, shoppyou->file);
    uint64_t size = shoppyou->size - shoppyou->hole_cnt;
    fwrite(&size, 8, 1, shoppyou->file);
    fflush(shoppyou->file);
    for (uint64_t i = 0; i < shoppyou->size; i++) {
        if (shoppyou->database[i].sasa_id != HOLE_ID && shoppyou->database[i].tanzaku_id != HOLE_ID) {
            fwrite(&shoppyou->database[i].created_ts, 8, 1, shoppyou->file);
            fwrite(&shoppyou->database[i].sasa_id, 8, 1, shoppyou->file);
            fwrite(&shoppyou->database[i].tanzaku_id, 8, 1, shoppyou->file);
        }
    }
    fflush(shoppyou->file);
    return 0;
}

int shoppyou_open(Shoppyou *shoppyou, const char *path) {
    shoppyou->file = fopen(path, "r+b");
    if (shoppyou->file == NULL) {
        fprintf(stderr, "Failed to dump shoppyou: failed to open file '%s'\n", path);
        return 1;
    }
    return shoppyou_load(shoppyou);
}

int shoppyou_dump(Shoppyou *shoppyou, const char *path) {
    shoppyou->file = fopen(path, "w+b");
    if (shoppyou->file == NULL) {
        fprintf(stderr, "Failed to dump shoppyou: failed to open file '%s'\n", path);
        return 1;
    }
    return shoppyou_save(shoppyou);
}

int kazari_add(Shoppyou *shoppyou, uint64_t sasa_id, uint64_t tanzaku_id) {
    if (sasa_id == HOLE_ID || tanzaku_id == HOLE_ID) {
        fprintf(stderr, "Failed to add kazari: got hole ID\n");
        return 1;
    }
    if (shoppyou->size == -1 && shoppyou->hole_cnt == 0) {
        fprintf(stderr, "Failed to add kazari: shoppyou is full\n");
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
        fprintf(stderr, "Failed to remove kazari: got hole ID\n");
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
    fprintf(stderr, "Failed to remove kazari: target kazari does not exist or is already removed\n");
    return 1;
}
